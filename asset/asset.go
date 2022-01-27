package asset

import (
	"embed"
	"io/fs"
)

//go:generate ./babel.sh

//go:embed data
var data embed.FS

func Data() fs.FS {
	data1, err := fs.Sub(data, "data")
	if err != nil {
		panic(err)
	}
	return data1
}
