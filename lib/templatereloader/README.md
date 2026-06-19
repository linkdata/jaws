# templatereloader

A templatereloader is a `jaws.TemplateLookuper` that will reload templates from
disk as needed if running with `-tags debug` or `-race`. If not, it simply calls
`template.New("").ParseFS(fsys, fpath)` and returns the parsed `*template.Template`
directly, adding no per-lookup indirection.

For example usage, see `jawsboot/README.md`
