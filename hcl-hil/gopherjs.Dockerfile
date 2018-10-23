FROM golang:1.11

RUN go get -u github.com/gopherjs/gopherjs
RUN go get -u github.com/hashicorp/hcl
RUN go get -u github.com/hashicorp/hil
RUN go get -u github.com/hashicorp/terraform
WORKDIR $GOPATH/src/github.com/hashicorp/terraform
RUN git remote add sjwl https://github.com/sjwl/terraform.git
RUN git pull --rebase sjwl master
WORKDIR /build
ADD main.go ./
RUN GOOS=darwin gopherjs build main.go -o build.js -v

CMD ["cat", "build.js"]