FROM golang:1.11

RUN go get -u github.com/gopherjs/gopherjs
RUN go get -u github.com/hashicorp/hcl
RUN go get -u github.com/hashicorp/hil
RUN go get -u github.com/hashicorp/terraform/terraform

ADD main.go ./
RUN GOOS=darwin gopherjs build main.go -o build.js -v

CMD ["cat", "build.js"]