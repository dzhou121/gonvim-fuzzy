let s:stop_chars = [
            \"\<Esc>",
            \"\<C-c>",
            \]

let s:confirm_chars = [
            \"\<Enter>",
            \"\<C-m>",
            \]

let s:up_chars = [
            \"\<C-k>",
            \]

let s:down_chars = [
            \"\<C-j>",
            \]

let s:right_chars = [
            \"\<rt>",
            \]

let s:left_chars = [
            \"\<lt>",
            \]

let s:del_chars = [
            \"\<del>",
            \]

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

function! nvim_fzf_shim#run(options)
    call rpcnotify(0, "FzfShim", "run", a:options)
    while v:true
        let s:input = getchar()
        let s:char = nr2char(s:input)

        let event = get(s:keymaps, s:char, 'noevent')
        if (s:input is# "\<BS>")
            call rpcnotify(0, "FzfShim", "backspace")
        elseif (s:input is# "\<DEL>")
            call rpcnotify(0, "FzfShim", "del")
        elseif (event == "noevent")
            call rpcnotify(0, "FzfShim", "char", s:char)
        else
            call rpcnotify(0, "FzfShim", event)
        endif

        if (event == "cancel") || (event == "confirm")
            return
        endif
    endwhile
endfunction

function! nvim_fzf_shim#exec(options)
    let s:arg = a:options.arg
    if has_key(a:options, 'function')
        let s:f = function(a:options.function)
        echo s:f(s:arg)
    endif
endfunction

function! s:buffer_line_handler(lines)
  execute split(a:lines, '\t')[0]
  normal! ^zz
endfunction

function! s:buffer_lines()
  return map(getline(1, "$"),
    \ 'printf("%d\t%s", v:key + 1, v:val)')
endfunction

function! nvim_fzf_shim#buffer_lines(...)
  let [query, args] = (a:0 && type(a:1) == type('')) ?
        \ [a:1, a:000[1:]] : ['', a:000]
  return nvim_fzf_shim#run({
              \ 'source': s:buffer_lines(),
              \ 'function': '<sid>buffer_line_handler',
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
function! nvim_fzf_shim#ag(query)
  let s:cmd = printf('%s "%s"', "ag --nogroup --column --nocolor", a:query)
  return nvim_fzf_shim#run({
              \ 'source': s:cmd,
              \ 'function': '<sid>ag_handler',
              \ "pwd": getcwd(),
              \ "max": 20, 
              \ "type": "ag", 
              \})
endfunction
