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
            \"\<Tab>": "down",
            \"\<C-k>": "up",
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
