## sfntshape

[![Go Reference](https://pkg.go.dev/badge/github.com/tinne26/sfntshape.svg)](https://pkg.go.dev/github.com/tinne26/sfntshape)

A simple `Shape` type to render vectorial shapes through [golang.org/x/image/vector#Rasterizer](https://pkg.go.dev/golang.org/x/image/vector#Rasterizer).

The `Shape` type offers two APIs:
- sfnt/Ebitengine style: `MoveTo`, `LineTo`, `QuadTo`, `CubeTo`.
- Relative style: `Turn`, `TurnTo`, `Move`, `Line`.

This code was originally part of [etxt](https://github.com/tinne26/etxt), but it was split after v0.0.9. The code now doesn't depend on [etxt](https://github.com/tinne26/etxt) nor [Ebitengine](https://ebitengine.org).

Check out a cool [online example](https://tinne26.github.io/sfntshape), or run it yourself with:
```
go run github.com/tinne26/sfntshape/examples/rng@latest
```
*(Instructions can be hidden with H and fullscreen can be switched with F)*
