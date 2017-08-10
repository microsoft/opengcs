#!/bin/sh

# Under travis, fgt golint ./... always returns zero, hence do it file by file
# Note we use fgt as golint exit code is always zero, even on failures, by design.
for f in *.go 
do
    fgt golint $f
    if [ $? -ne 0 ]; then
        echo "ERROR: Go code is not linted:"
        exit 1
    fi
done
echo "All .go files are correctly linted!"

if [ -n "$(gofmt -s -l .)" ]; then
    echo "ERROR: Go code is not formatted:"
    gofmt -s -d -e .
    exit 1
fi
echo "All .go files are correctly formatted!"

