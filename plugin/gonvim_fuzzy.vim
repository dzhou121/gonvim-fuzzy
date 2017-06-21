command! -nargs=? GonvimFuzzyFiles call gonvim_fuzzy#files(<q-args>, <bang>0)
command! -nargs=? GonvimFuzzyBLines call gonvim_fuzzy#buffer_lines(<q-args>, <bang>0)
