#!/bin/bash

repo=$(basename $(pwd))
if [ -f main.go -o -f ${repo}.go ] ; then
    echo "ready to build: ${repo} ${GOARCH}"
else
    echo "Fail: execute ${0} under repo directory"
    exit 1
fi

outdir="build"
[ -d ${outdir} ] || mkdir -p ${outdir}

outpath="${outdir}/${repo}"

last_md5=$(md5sum ${outpath} 2>/dev/null | awk '{print $1}')

export GOPROXY="https://goproxy.cn,direct"
go mod tidy
go mod vendor

#CC=/opt/platforms/linux/x86_64-pc-linux-gnu/bin/x86_64-pc-linux-gnu-gcc CGO_ENABLED=1
CGO_ENABLED=0 go build -ldflags "-s -w" -v -o ${outpath} || {
    echo "============build failed ========="
    exit 1
}

echo
new_md5=$(md5sum ${outpath} 2>/dev/null)
if echo ${new_md5} | grep ${last_md5} >/dev/null; then
    echo "============build not changed ========="
    echo "last_md5=${last_md5}"
    echo "new_md5=${new_md5}"
    exit 0
fi

echo "============build done========="
ls -l ${outpath}
md5sum ${outpath}
