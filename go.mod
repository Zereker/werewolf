module github.com/Zereker/werewolf

go 1.23

require github.com/Zereker/hiddenrole v0.0.0

// 引擎已独立成一个 module。这条 replace 让本仓库在**不发版**的情况下
// 就能对着本地的引擎源码编译与跑测试——三套规则包与引擎的改动往往要
// 一起验，隔着一次发版做不了。
//
// 引擎那边真正发版之后，把版本号填进上面的 require、删掉这一行即可。
replace github.com/Zereker/hiddenrole => ./hiddenrole
