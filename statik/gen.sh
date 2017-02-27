#!/bin/sh

for i in *.js
do
	babel $i -o data/js/$i
done
go generate
