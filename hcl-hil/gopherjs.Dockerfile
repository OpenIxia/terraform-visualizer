FROM golang:1.11

# WORKDIR $GOPATH/src/github.com/hashicorp/terraform
# RUN git remote add sjwl https://github.com/sjwl/terraform.git
# RUN git pull --rebase sjwl master

ADD . $GOPATH/src/github.com/openixia/terraform-visualizer/hcl-hil
WORKDIR $GOPATH/src/github.com/openixia/terraform-visualizer/hcl-hil
RUN go get -u github.com/kardianos/govendor
RUN govendor sync -v

RUN go get -u github.com/gopherjs/gopherjs
RUN GOOS=darwin gopherjs build main.go -o build.js -v

CMD ["cat", "build.js"]