package main

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"

	"github.com/rajveermalviya/go-webgpu/wgpu"
)

type Texture struct {
	texture *wgpu.Texture
	view    *wgpu.TextureView
	sampler *wgpu.Sampler
}

func (t *Texture) Destroy() {
	if t.sampler != nil {
		t.sampler.Release()
		t.sampler = nil
	}
	if t.view != nil {
		t.view.Release()
		t.view = nil
	}
	if t.texture != nil {
		t.texture.Release()
		t.texture = nil
	}
}

func TextureFromPNGBytes(device *wgpu.Device, queue *wgpu.Queue, buf []byte, label string) (*Texture, error) {
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	return TextureFromImage(device, queue, img, label)
}

func TextureFromImage(device *wgpu.Device, queue *wgpu.Queue, img image.Image, label string) (t *Texture, err error) {
	defer func() {
		if err != nil {
			t.Destroy()
			t = nil
		}
	}()
	t = &Texture{}

	r := img.Bounds()
	width := r.Dx()
	height := r.Dy()

	// Convert to RGBA
	rgbaImg, ok := img.(*image.RGBA)
	if !ok {
		rgbaImg = image.NewRGBA(r)
		draw.Draw(rgbaImg, r, img, image.Point{}, draw.Over)
	}

	size := wgpu.Extent3D{
		Width:              uint32(width),
		Height:             uint32(height),
		DepthOrArrayLayers: 1,
	}
	t.texture, err = device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         label,
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension_2D,
		Format:        wgpu.TextureFormat_RGBA8UnormSrgb,
		Usage:         wgpu.TextureUsage_TextureBinding | wgpu.TextureUsage_CopyDst,
	})
	if err != nil {
		return
	}

	queue.WriteTexture(
		&wgpu.ImageCopyTexture{
			Aspect:   wgpu.TextureAspect_All,
			Texture:  t.texture,
			MipLevel: 0,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
		},
		rgbaImg.Pix,
		&wgpu.TextureDataLayout{
			Offset:       0,
			BytesPerRow:  4 * uint32(width),
			RowsPerImage: uint32(height),
		},
		&size,
	)

	t.view, err = t.texture.CreateView(nil)
	if err != nil {
		return
	}

	t.sampler, err = device.CreateSampler(nil)
	if err != nil {
		return
	}

	return t, nil
}
