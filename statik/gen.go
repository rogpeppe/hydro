package statik

// TODO make the "statik" tool general enough that we can call this
// package "static" not "static"

//go:generate sh -c "statik -compress -src data && mv statik/*.go . && rmdir statik"
