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
            \"\<Del>": "del",
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

map <silent> <leader>fs :call nvim_fzf_shim#run({
            \"source": "python ~/dotfiles/dir.py",
            \"sink": "e",
            \"function": "<sid>abc",
            \"max": 10, 
            \})<cr>
