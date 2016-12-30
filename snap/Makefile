all: ./bin/armhf/hydroserver ./bin/amd64/hydroserver
install: all
	test -d ${DESTDIR} || (echo DESTDIR not set; exit 1)
	cp -r ./bin ${DESTDIR}

PKG=github.com/rogpeppe/hydro/cmd/hydroserver

./bin/armhf/hydroserver:
	GOARCH=arm go build -o $@ ${PKG}

./bin/amd64/hydroserver:
	GOARCH=amd64 go build -o $@ ${PKG}
