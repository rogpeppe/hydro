.PHONY: all install

all:
	GOARCH=arm go install ${PKG}
	GOARCH=arm go build -o ./bin/armhf/hydroserver ${PKG}
	GOARCH=amd64 go install ${PKG}
	GOARCH=amd64 go build -o ./bin/amd64/hydroserver ${PKG}

# note: the install target is used by snapcraft build
install: all
	test -d ${DESTDIR} || (echo DESTDIR not set; exit 1)
	cp -r ./bin ${DESTDIR}

PKG=github.com/rogpeppe/hydro/cmd/hydroserver
