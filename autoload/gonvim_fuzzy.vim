let s:keymaps = {
            \"\<Esc>": "cancel",
            \"\<C-c>": "cancel",
            \"\<Enter>": "confirm",
            \"\kb": "backspace",
            \"\<C-h>": "backspace",
            \"\<C-b>": "left",
            \"\<C-f>": "right",
            \"\<C-u>": "clear",
            \"\<Del>": "del",
            \"\<C-j>": "down",
            \"\<C-n>": "down",
            \"\<Tab>": "down",
            \"\<C-k>": "up",
            \"\<C-p>": "up",
            \}

function! gonvim_fuzzy#run(options)
    call rpcnotify(0, "GonvimFuzzy", "run", a:options)
    while v:true
        let s:input = getchar()
        let s:char = nr2char(s:input)

        let event = get(s:keymaps, s:char, 'noevent')
        if (s:input is# "\<BS>")
            call rpcnotify(0, "GonvimFuzzy", "backspace")
        elseif (s:input is# "\<DEL>")
            call rpcnotify(0, "GonvimFuzzy", "del")
        elseif (event == "noevent")
            call rpcnotify(0, "GonvimFuzzy", "char", s:char)
        else
            call rpcnotify(0, "GonvimFuzzy", event)
        endif

        if (event == "cancel") || (event == "confirm")
            return
        endif
    endwhile
endfunction

function! gonvim_fuzzy#exec(options)
    let s:arg = a:options.arg
    if has_key(a:options, 'function')
        let s:f = function(a:options.function)
        echo s:f(s:arg)
    endif
endfunction

function! s:warn(message)
  echohl WarningMsg
  echom a:message
  echohl None
  return 0
endfunction

" ------------------------------------------------------------------
" Files
" ------------------------------------------------------------------
function! s:edit_file(item)
  execute 'e' a:item
endfunction

function! gonvim_fuzzy#files(dir, ...)
  let s:dir = ""
  if !empty(a:dir)
    let s:dir = expand(a:dir)
    if !isdirectory(s:dir)
      return s:warn('Invalid directory')
    endif
  endif

  return gonvim_fuzzy#run({
              \ 'function': 's:edit_file',
              \ "pwd": getcwd(),
              \ "dir": s:dir,
              \ "type": "file", 
              \})
endfunction

" ------------------------------------------------------------------
" Buffer Lines
" ------------------------------------------------------------------
function! s:buffer_line_handler(lines)
  execute split(a:lines, '\t')[0]
  normal! ^zz
endfunction

function! s:buffer_lines()
  return map(getline(1, "$"),
    \ 'printf("%d\t%s", v:key + 1, v:val)')
endfunction

function! gonvim_fuzzy#buffer_lines(...)
  let [query, args] = (a:0 && type(a:1) == type('')) ?
        \ [a:1, a:000[1:]] : ['', a:000]
  return gonvim_fuzzy#run({
              \ 'source': s:buffer_lines(),
              \ 'function': 's:buffer_line_handler',
              \ "pwd": getcwd(),
              \ "type": "line",
              \ "max": 20, 
              \})
endfunction

" ------------------------------------------------------------------
" Ag
" ------------------------------------------------------------------
function! s:ag_to_qf(line, with_column)
  let parts = split(a:line, ':')
  let text = join(parts[(a:with_column ? 3 : 2):], ':')
  let dict = {'filename': &acd ? fnamemodify(parts[0], ':p') : parts[0], 'lnum': parts[1], 'text': text}
  if a:with_column
    let dict.col = parts[2]
  endif
  return dict
endfunction

function! s:ag_handler(line)
  let s:parts = split(a:line, ':')
  execute "e" s:parts[0]
  execute s:parts[1]
  normal! ^zz
  execute 'normal!' s:parts[2].'|'
endfunction

" query, [[ag options], options]
function! gonvim_fuzzy#ag(query, ...)
  let s:cmd = printf('%s "%s"', "ag --nogroup --column --nocolor", a:query)
  return gonvim_fuzzy#run({
              \ 'source': s:cmd,
              \ 'function': 's:ag_handler',
              \ "pwd": getcwd(),
              \ "max": 20, 
              \ "type": "file_line", 
              \})
endfunction

" ------------------------------------------------------------------
" Buffers
" ------------------------------------------------------------------
function! s:find_open_window(b)
  let [tcur, tcnt] = [tabpagenr() - 1, tabpagenr('$')]
  for toff in range(0, tabpagenr('$') - 1)
    let t = (tcur + toff) % tcnt + 1
    let buffers = tabpagebuflist(t)
    for w in range(1, len(buffers))
      let b = buffers[w - 1]
      if b == a:b
        return [t, w]
      endif
    endfor
  endfor
  return [0, 0]
endfunction

function! s:jump(t, w)
  execute 'normal!' a:t.'gt'
  execute a:w.'wincmd w'
endfunction

function! s:bufopen(lines)
  if len(a:lines) < 2
    return
  endif
  let b = matchstr(a:lines[1], '\[\zs[0-9]*\ze\]')
  if empty(a:lines[0]) && get(g:, 'fzf_buffers_jump')
    let [t, w] = s:find_open_window(b)
    if t
      call s:jump(t, w)
      return
    endif
  endif
  let cmd = get(get(g:, 'fzf_action', s:default_action), a:lines[0], '')
  if !empty(cmd)
    execute 'silent' cmd
  endif
  execute 'buffer' b
endfunction

function! s:format_buffer(b)
  let name = bufname(a:b)
  let name = empty(name) ? '[No Name]' : fnamemodify(name, ":~:.")
  let flag = a:b == bufnr('')  ? s:blue('%', 'Conditional') :
          \ (a:b == bufnr('#') ? s:magenta('#', 'Special') : ' ')
  let modified = getbufvar(a:b, '&modified') ? s:red(' [+]', 'Exception') : ''
  let readonly = getbufvar(a:b, '&modifiable') ? '' : s:green(' [RO]', 'Constant')
  let extra = join(filter([modified, readonly], '!empty(v:val)'), '')
  return s:strip(printf("[%s] %s\t%s\t%s", s:yellow(a:b, 'Number'), flag, name, extra))
endfunction

function! s:sort_buffers(...)
  let [b1, b2] = map(copy(a:000), 'get(g:fzf#vim#buffers, v:val, v:val)')
  " Using minus between a float and a number in a sort function causes an error
  return b1 > b2 ? 1 : -1
endfunction

function! gonvim_fuzzy#buffers(...)
  let s:bufs = map(sort(s:buflisted(), 's:sort_buffers'), 's:format_buffer(v:val)')

  " let [query, args] = (a:0 && type(a:1) == type('')) ?
  "       \ [a:1, a:000[1:]] : ['', a:000]
  return gonvim_fuzzy#run({
              \ 'source': reverse(s:bufs),
              \ 'function': 's:bufopen',
              \ "type": "file", 
              \})
endfunction
