# node-syscall

This takes gopherjs node-syscall version at <https://github.com/gopherjs/gopherjs/tree/master/node-syscall> into a separate project with a package.json and provides downloadable binary assets.

The use case for this was for creating a VSCode extension that relied on Gopherjs *and* making system calls, that didn't rely on the end user (the VSCode user) to compile and copy `syscall.node` to their extension's `node_module` directory.
