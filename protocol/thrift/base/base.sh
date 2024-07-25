#! /bin/bash

set -e

thriftgo -g fastgo:no_default_serdes=true -o=.. ./base.thrift

mv base.go base_tmp.go # fix sed base.go > base.go
# rm unused funcs and vars, keep the file smaller:
# func GetXXX
# func IsSet
# multiline DEFAULT vars
# singleline DEFAULT vars
sed '/func.* Get.* {/,/^}/d' base_tmp.go |\
  sed '/func.* IsSet.* {/,/^}/d' |\
  sed '/DEFAULT.*{/,/^}/d' |\
  sed '/DEFAULT/d' > base.go

gofmt -w base.go
rm base_tmp.go
