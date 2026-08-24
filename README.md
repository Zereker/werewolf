# werewolf

> **这个仓库的代码已经搬到 [Zereker/hiddenrole](https://github.com/Zereker/hiddenrole)。**
> **This repository has moved to [Zereker/hiddenrole](https://github.com/Zereker/hiddenrole).**

狼人杀规则包现在是 `hiddenrole` 这个 module 里的一个包，和另外两套规则包
平级住在 `games/` 下：

| 原来 | 现在 |
|---|---|
| `github.com/Zereker/werewolf` | [`github.com/Zereker/hiddenrole/example/werewolf`](https://github.com/Zereker/hiddenrole/tree/master/example/werewolf) |
| `github.com/Zereker/werewolf/missions` | [`github.com/Zereker/hiddenrole/example/missions`](https://github.com/Zereker/hiddenrole/tree/master/example/missions) |
| `github.com/Zereker/werewolf/onenight` | [`github.com/Zereker/hiddenrole/example/onenight`](https://github.com/Zereker/hiddenrole/tree/master/example/onenight) |

```sh
go get github.com/Zereker/hiddenrole
```

```go
import "github.com/Zereker/hiddenrole/example/werewolf"
```

## 为什么搬

内核（`hiddenrole`）与规则包本来是两个仓库，而 `werewolf` 这个名字只覆盖
三套规则包中的一套。结构于是同时说两件互相矛盾的话：「这是狼人杀库」与
「这是三套平级的规则包」。合并之后，内核在根、三套规则包平级放在 `games/`
下，谁也不比谁高一级。

完整的说明、更新日志、设计文档与全部历史都在新仓库。已经发布的 tag
（`v1.0.0`、`v1.2.0`）仍然可以取用，但不再更新。

## 许可证

MIT License，见 [LICENSE](LICENSE)。
