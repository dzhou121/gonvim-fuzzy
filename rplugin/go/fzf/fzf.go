package fzf

import (
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
	"github.com/neovim/go-client/nvim"
)

const (
	slab16Size int = 100 * 1024 // 200KB * 32 = 12.8MB
	slab32Size int = 2048       // 8KB * 32 = 256KB
)

// Shim is
type Shim struct {
	nvim        *nvim.Nvim
	options     map[string]interface{}
	source      []string
	sourceNew   chan string
	max         int
	selected    int
	pattern     string
	cursor      int
	slab        *util.Slab
	start       int
	result      []*Output
	scoreMutext *sync.Mutex
	scoreNew    bool
	cancelled   bool
	cancelChan  chan bool
}

// Output is
type Output struct {
	result algo.Result
	match  *[]int
	output string
}

// RegisterPlugin registers this remote plugin
func RegisterPlugin(nvim *nvim.Nvim) {
	nvim.Subscribe("FzfShim")
	shim := &Shim{
		nvim:        nvim,
		slab:        util.MakeSlab(slab16Size, slab32Size),
		scoreMutext: &sync.Mutex{},
	}
	nvim.RegisterHandler("FzfShim", func(args ...interface{}) {
		if len(args) < 1 {
			return
		}
		event, ok := args[0].(string)
		if !ok {
			return
		}
		switch event {
		case "run":
			go shim.run(args[1:])
		case "char":
			go shim.newChar(args[1:])
		case "backspace":
			go shim.backspace()
		case "left":
			go shim.left()
		case "right":
			go shim.right()
		case "down":
			go shim.down()
		case "up":
			go shim.up()
		case "cancel":
			go shim.cancel()
		default:
			fmt.Println("unhandleld fzfshim event", event)
		}
	})
}

func (s *Shim) run(args []interface{}) {
	ok := s.parseOptions(args)
	if !ok {
		return
	}
	fmt.Println(s.options)
	s.reset()
	s.processMax()
	s.processSource()
	s.outputShow()
	s.outputPattern()
	s.outputCursor()
	s.filter()
}

func (s *Shim) reset() {
	s.source = []string{}
	s.selected = 0
	s.max = 0
	s.pattern = ""
	s.cursor = 0
	s.start = 0
	s.sourceNew = make(chan string, 1000)
	s.cancelled = false
	s.cancelChan = make(chan bool, 1)
}

// ByScore sorts the output by score
type ByScore []*Output

func (a ByScore) Len() int {
	return len(a)
}

func (a ByScore) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a ByScore) Less(i, j int) bool {
	iout := a[i]
	jout := a[j]
	return (iout.result.Score < jout.result.Score)
}

func (s *Shim) filter() {
	if s.cancelled {
		return
	}
	s.scoreNew = true
	s.scoreMutext.Lock()
	defer s.scoreMutext.Unlock()
	s.scoreNew = false
	s.result = []*Output{}
	sourceNew := s.sourceNew

	stop := make(chan bool)
	go func() {
		tick := time.Tick(100 * time.Millisecond)
		for {
			select {
			case <-tick:
				s.outputResult()
			case <-stop:
				return
			}
		}
	}()
	defer func() {
		stop <- true
	}()

	for _, source := range s.source {
		s.scoreSource(source)
		if s.scoreNew || s.cancelled {
			return
		}
	}
	for source := range sourceNew {
		s.source = append(s.source, source)
		s.scoreSource(source)
		if s.scoreNew || s.cancelled {
			return
		}
	}
	s.outputResult()
}

func (s *Shim) scoreSource(source string) {
	r := algo.Result{
		Score: -1,
	}
	n := &[]int{}

	if s.pattern != "" {
		chars := util.ToChars([]byte(source))
		r, n = algo.FuzzyMatchV2(false, true, true, chars, []rune(s.pattern), true, s.slab)
	}
	if r.Score == -1 || r.Score > 0 {
		i := 0
		if r.Score > 0 {
			for i = 0; i < len(s.result); i++ {
				if s.result[i].result.Score < r.Score {
					break
				}
			}
		} else {
			i = len(s.result)
		}

		s.result = append(s.result[:i],
			append(
				[]*Output{&Output{
					result: r,
					output: source,
					match:  n,
				}},
				s.result[i:]...)...)

		// if s.start <= i && i <= s.start+s.max-1 {
		// 	s.outputResult()
		// }
	}
}

func (s *Shim) processSource() {
	source, ok := s.options["source"]
	if !ok {
		return
	}
	sourceNew := s.sourceNew
	cancelChan := s.cancelChan
	switch src := source.(type) {
	case []interface{}:
		go func() {
			for _, item := range src {
				if s.cancelled {
					close(sourceNew)
					return
				}
				str, ok := item.(string)
				if !ok {
					continue
				}

				select {
				case sourceNew <- str:
				case <-cancelChan:
					close(sourceNew)
					return
				}
			}
			close(sourceNew)
		}()
	case string:
		cmd := exec.Command("bash", "-c", src)
		stdout, _ := cmd.StdoutPipe()
		output := ""
		go func() {
			buf := make([]byte, 2)
			for {
				n, err := stdout.Read(buf)
				if err != nil || s.cancelled {
					close(sourceNew)
					stdout.Close()
					cmd.Wait()
					return
				}
				output += string(buf[0:n])
				parts := strings.Split(output, "\n")
				if len(parts) > 1 {
					for i := 0; i < len(parts)-1; i++ {
						// s.source = append(s.source, parts[i])
						select {
						case sourceNew <- parts[i]:
						case <-cancelChan:
							close(sourceNew)
							stdout.Close()
							cmd.Wait()
							return
						}
					}
					output = parts[len(parts)-1]
				}
			}
		}()
		cmd.Start()
	default:
		fmt.Println(reflect.TypeOf(source))
	}
}

func (s *Shim) processMax() {
	max, ok := s.options["max"]
	if !ok {
		return
	}
	m, ok := max.(int64)
	s.max = int(m)
}

func (s *Shim) parseOptions(args []interface{}) bool {
	if len(args) == 0 {
		return false
	}
	options, ok := args[0].(map[string]interface{})
	if !ok {
		return false
	}
	s.options = options
	return true
}

func (s *Shim) newChar(args []interface{}) {
	if len(args) == 0 {
		return
	}
	c, ok := args[0].(string)
	if !ok {
		return
	}
	if len(c) == 0 {
		return
	}
	s.pattern = insertAtIndex(s.pattern, s.cursor, c)
	s.cursor++
	s.outputPattern()
	s.outputCursor()
	s.filter()
}

func (s *Shim) backspace() {
	fmt.Println("backspace")
	if s.cursor == 0 {
		return
	}
	s.cursor--
	s.pattern = removeAtIndex(s.pattern, s.cursor)
	s.outputPattern()
	s.outputCursor()
	s.filter()
}

func (s *Shim) left() {
	if s.cursor > 0 {
		s.cursor--
	}
	s.outputCursor()
}

func (s *Shim) outputPattern() {
	s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_pattern", s.pattern)
}

func (s *Shim) outputShow() {
	s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_show")
}

func (s *Shim) outputHide() {
	s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_hide")
}

func (s *Shim) outputCursor() {
	s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_pattern_pos", s.cursor)
}

func (s *Shim) outputResult() {
	if s.start >= len(s.result) {
		s.start = 0
		s.selected = 0
	}
	end := s.start + s.max
	if end > len(s.result) {
		end = len(s.result)
	}
	output := []string{}
	match := [][]int{}
	for _, o := range s.result[s.start:end] {
		output = append(output, o.output)
	}
	for _, o := range s.result[s.start:end] {
		if o.match == nil {
			match = append(match, []int{})
		} else {
			match = append(match, *o.match)
		}
	}

	s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_show_result", output, s.selected-s.start, match)
}

func (s *Shim) right() {
	if s.cursor < len(s.pattern) {
		s.cursor++
	}
	s.outputCursor()
}

func (s *Shim) up() {
	if s.selected > 0 {
		s.selected--
	} else if s.selected == 0 {
		s.selected = len(s.result) - 1
	}
	s.processSelected()
}

func (s *Shim) down() {
	if s.selected < len(s.result)-1 {
		s.selected++
	} else if s.selected == len(s.result)-1 {
		s.selected = 0
	}
	s.processSelected()
}

func (s *Shim) processSelected() {
	if s.selected < s.start {
		s.start = s.selected
		s.outputResult()
	} else if s.selected >= s.start+s.max {
		s.start = s.selected - s.max + 1
		s.outputResult()
	}
	s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_select", s.selected-s.start)
}

func (s *Shim) cancel() {
	fmt.Println("stop")
	s.outputHide()
	s.cancelled = true
	s.cancelChan <- true
	s.reset()
}

func removeAtIndex(in string, i int) string {
	if len(in) == 0 {
		return in
	}
	if i >= len(in) {
		return in
	}
	a := []rune(in)
	a = append(a[:i], a[i+1:]...)
	return string(a)
}

func insertAtIndex(in string, i int, newChar string) string {
	a := []rune(in)
	a = append(a[:i], append([]rune{rune(newChar[0])}, a[i:]...)...)
	return string(a)
}
