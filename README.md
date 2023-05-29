# go-webgpu-examples

`go-webgpu` is Go bindings for [`wgpu-native`](https://github.com/gfx-rs/wgpu-native), a cross-platform, safe, graphics api. It runs natively on Vulkan, Metal, D3D12.

## Examples

### [compute](./compute/main.go)

```shell
go run github.com/rajveermalviya/go-webgpu-examples/compute@latest
```

### [capture](./capture/main.go)

Creates `./image.png` with all pixels red and size 100x200

```shell
go run github.com/rajveermalviya/go-webgpu-examples/capture@latest
```

### [triangle](./triangle/main.go)

This example uses [go-glfw](https://github.com/go-gl/glfw) so it will use cgo on **_all platforms_**, you will also need
[some libraries installed](https://github.com/go-gl/glfw#installation) to run the example.

```shell
go run github.com/rajveermalviya/go-webgpu-examples/triangle@latest

# same example but with 4x MSAA
go run github.com/rajveermalviya/go-webgpu-examples/triangle-msaa@latest
```

![](./triangle/image-msaa.png)

### [cube](./cube/main.go)

This example also uses [go-glfw](https://github.com/go-gl/glfw).

```shell
go run github.com/rajveermalviya/go-webgpu-examples/cube@latest
```

![](./cube/image-msaa.png)

### [boids](./boids/main.go)

This example also uses [go-glfw](https://github.com/go-gl/glfw).

```shell
go run github.com/rajveermalviya/go-webgpu-examples/boids@latest
```

![](./boids/image-msaa.png)

### [gamen-windowing](./gamen-windowing/main.go)

This example uses [gamen](https://github.com/rajveermalviya/gamen) for windowing, it **doesn't** use cgo on windows. On linux you may need to [install some packages](https://github.com/rajveermalviya/gamen#linux).

```shell
go run github.com/rajveermalviya/go-webgpu-examples/gamen-windowing@latest
```

This example also supports running on android.

```shell
# install android sdk
# connect your device and setup adb / run android emulator

# install tsukuru to build apk
go install github.com/rajveermalviya/tsukuru/cmd/tsukuru@latest

cd examples/gamen-windowing
tsukuru run apk .
```
