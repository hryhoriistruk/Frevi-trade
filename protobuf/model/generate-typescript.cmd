# Usage: generate-react.cmd  <protofile name> 
PROTOFILE=$1
OUT=../../react/opentp-client/src/serverapi
protoc $PROTOFILE --js_out=import_style=commonjs,binary:$OUT --grpc-web_out=import_style=typescript,mode=grpcwebtext:$OUT --proto_path=.
protoc ./*.proto --js_out=import_style=commonjs,binary:$OUT --grpc-web_out=import_style=typescript,mode=grpcwebtext:$OUT --proto_path=.

# A workaround in the typescript plugin for grpcweb 
for f in $OUT/*.js 
do
 if !(grep -q "eslint-disable" $f) 
 then
    sed -i '1s/^/\/* eslint-disable *\/\n/' $f
 fi
done



