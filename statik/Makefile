TARGET=data/js/home.js data/js/reconnecting-websocket.js

all: ${TARGET}

data/js/%.js: ./%.js .babelrc
	babel $< -o $@
