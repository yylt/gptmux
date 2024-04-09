#!/bin/bash

ff=$(find . -type f -name "*.go" -print)

for f in ${ff}; do
	c=$(sed -n 1p $f |grep -ci Copyright)
	if [ $c -ne 0 ] ;then
		echo 'had add copyright :' $f
        else
		echo 'add copyright: ' $f
        sed -i '1i \// Copyright 2022 yylt.' $f
	fi
done
