package fzf

import (
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"sync"

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
	selected    uint64
	pattern     string
	cursor      int
	slab        *util.Slab
	start       int
	result      []*Output
	scoreMutext *sync.Mutex
	scoreNew    bool
}

// Output is
type Output struct {
	result algo.Result
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
}

func (s *Shim) reset() {
	s.source = []string{}
	s.selected = 0
	s.max = 0
	s.pattern = ""
	s.cursor = 0
	s.start = 0
	s.sourceNew = make(chan string, 1000)
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

func (s *Shim) show() {
	s.scoreNew = true
	s.scoreMutext.Lock()
	defer s.scoreMutext.Unlock()
	s.scoreNew = false
	s.result = []*Output{}
	for _, source := range s.source {
		s.scoreSource(source)
		if s.scoreNew {
			return
		}
	}
	for source := range s.sourceNew {
		s.source = append(s.source, source)
		s.scoreSource(source)
		if s.scoreNew {
			return
		}
	}
}

func (s *Shim) scoreSource(source string) {
	chars := util.ToChars([]byte(source))
	r, _ := algo.FuzzyMatchV2(false, true, true, chars, []rune(s.pattern), true, s.slab)
	if r.Score > 0 {
		i := 0
		for i = 0; i < len(s.result); i++ {
			if s.result[i].result.Score < r.Score {
				break
			}
		}

		s.result = append(s.result[:i], append([]*Output{&Output{result: r, output: source}}, s.result[i:]...)...)

		if s.start <= i && i <= s.start+s.max-1 {
			end := s.start + s.max
			if end >= len(s.result) {
				end = len(s.result) - 1
			}
			// fmt.Println("-------------------------------------------------")
			// for _, o := range s.result[s.start:end] {
			// 	fmt.Println(o.output)
			// }
			// fmt.Println(i)
			// fmt.Println("-------------------------------------------------")
		}
	}
}

func (s *Shim) processSource() {
	source, ok := s.options["source"]
	if !ok {
		return
	}
	emit := false
	switch src := source.(type) {
	case []interface{}:
		for _, item := range src {
			str, ok := item.(string)
			if !ok {
				continue
			}
			s.source = append(s.source, str)
			if !emit {
				if len(s.source) >= s.max {
					emit = true
					s.show()
				}
			}
		}
		fmt.Println(s.source)
	case string:
		cmd := exec.Command("bash", "-c", src)
		stdout, _ := cmd.StdoutPipe()
		output := ""
		go func() {
			buf := make([]byte, 2)
			for {
				n, err := stdout.Read(buf)
				if err != nil {
					close(s.sourceNew)
					cmd.Wait()
					break
				}
				output += string(buf[0:n])
				parts := strings.Split(output, "\n")
				if len(parts) > 1 {
					for i := 0; i < len(parts)-1; i++ {
						// s.source = append(s.source, parts[i])
						s.sourceNew <- parts[i]
					}
					output = parts[len(parts)-1]
				}
			}
		}()
		cmd.Start()
	default:
		fmt.Println(reflect.TypeOf(source))
	}
	if !emit {
		s.show()
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
}

func (s *Shim) left() {
	if s.cursor > 0 {
		s.cursor--
	}
	s.outputCursor()
}

func (s *Shim) outputPattern() {
	fmt.Println(s.pattern)
}

func (s *Shim) outputCursor() {
	fmt.Println(s.cursor)
}

func (s *Shim) right() {
	if s.cursor < len(s.pattern) {
		s.cursor++
	}
	s.outputCursor()
}

func (s *Shim) cancel() {
	fmt.Println("stop")
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
