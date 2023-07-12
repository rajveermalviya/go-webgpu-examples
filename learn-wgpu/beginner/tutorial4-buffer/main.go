package main

import (
	_ "embed"
	"fmt"
	"strings"
	"unsafe"

	"github.com/rajveermalviya/gamen/display"
	"github.com/rajveermalviya/gamen/dpi"
	"github.com/rajveermalviya/go-webgpu/wgpu"
)

//go:embed shader.wgsl
var shaderCode string

type Vertex struct {
	position [3]float32
	color    [3]float32
}

var VertexBufferLayout = wgpu.VertexBufferLayout{
	ArrayStride: uint64(unsafe.Sizeof(Vertex{})),
	StepMode:    wgpu.VertexStepMode_Vertex,
	Attributes: []wgpu.VertexAttribute{
		{
			Offset:         0,
			ShaderLocation: 0,
			Format:         wgpu.VertexFormat_Float32x3,
		},
		{
			Offset:         uint64(unsafe.Sizeof([3]float32{})),
			ShaderLocation: 1,
			Format:         wgpu.VertexFormat_Float32x3,
		},
	},
}

var VERTICES = [...]Vertex{
	{
		position: [3]float32{-0.0868241, 0.49240386, 0.0},
		color:    [3]float32{0.5, 0.0, 0.5},
	}, // A
	{
		position: [3]float32{-0.49513406, 0.06958647, 0.0},
		color:    [3]float32{0.5, 0.0, 0.5},
	}, // B
	{
		position: [3]float32{-0.21918549, -0.44939706, 0.0},
		color:    [3]float32{0.5, 0.0, 0.5},
	}, // C
	{
		position: [3]float32{0.35966998, -0.3473291, 0.0},
		color:    [3]float32{0.5, 0.0, 0.5},
	}, // D
	{
		position: [3]float32{0.44147372, 0.2347359, 0.0},
		color:    [3]float32{0.5, 0.0, 0.5},
	}, // E
}

var INDICES = [...]uint16{0, 1, 4, 1, 2, 4, 2, 3, 4}

type State struct {
	surface        *wgpu.Surface
	swapChain      *wgpu.SwapChain
	device         *wgpu.Device
	queue          *wgpu.Queue
	config         *wgpu.SwapChainDescriptor
	size           dpi.PhysicalSize[uint32]
	renderPipeline *wgpu.RenderPipeline

	vertexBuffer *wgpu.Buffer
	indexBuffer  *wgpu.Buffer
	numIndices   uint32
}

func InitState(window display.Window) (s *State, err error) {
	defer func() {
		if err != nil {
			s.Destroy()
			s = nil
		}
	}()
	s = &State{}

	s.size = window.InnerSize()

	instance := wgpu.CreateInstance(nil)
	defer instance.Release()

	s.surface = instance.CreateSurface(getSurfaceDescriptor(window))

	adaper, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		CompatibleSurface: s.surface,
	})
	if err != nil {
		return s, err
	}
	defer adaper.Release()

	s.device, err = adaper.RequestDevice(nil)
	if err != nil {
		return s, err
	}
	s.queue = s.device.GetQueue()

	s.config = &wgpu.SwapChainDescriptor{
		Usage:       wgpu.TextureUsage_RenderAttachment,
		Format:      s.surface.GetPreferredFormat(adaper),
		Width:       s.size.Width,
		Height:      s.size.Height,
		PresentMode: wgpu.PresentMode_Fifo,
	}
	s.swapChain, err = s.device.CreateSwapChain(s.surface, s.config)
	if err != nil {
		return s, err
	}

	shader, err := s.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "shader.wgsl",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{
			Code: shaderCode,
		},
	})
	if err != nil {
		return s, err
	}
	defer shader.Release()

	renderPipelineLayout, err := s.device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label: "Render Pipeline Layout",
	})
	if err != nil {
		return s, err
	}
	defer renderPipelineLayout.Release()

	s.renderPipeline, err = s.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "Render Pipeline",
		Layout: renderPipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     shader,
			EntryPoint: "vs_main",
			Buffers:    []wgpu.VertexBufferLayout{VertexBufferLayout},
		},
		Fragment: &wgpu.FragmentState{
			Module:     shader,
			EntryPoint: "fs_main",
			Targets: []wgpu.ColorTargetState{{
				Format:    s.config.Format,
				Blend:     &wgpu.BlendState_Replace,
				WriteMask: wgpu.ColorWriteMask_All,
			}},
		},
		Primitive: wgpu.PrimitiveState{
			Topology:  wgpu.PrimitiveTopology_TriangleList,
			FrontFace: wgpu.FrontFace_CCW,
			CullMode:  wgpu.CullMode_Back,
		},
		Multisample: wgpu.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
	})
	if err != nil {
		return s, err
	}

	s.vertexBuffer, err = s.device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "Vertex Buffer",
		Contents: wgpu.ToBytes(VERTICES[:]),
		Usage:    wgpu.BufferUsage_Vertex,
	})
	if err != nil {
		return s, err
	}

	s.indexBuffer, err = s.device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "Index Buffer",
		Contents: wgpu.ToBytes(INDICES[:]),
		Usage:    wgpu.BufferUsage_Index,
	})
	if err != nil {
		return s, err
	}
	s.numIndices = uint32(len(INDICES))

	return s, nil
}

func (s *State) Resize(newSize dpi.PhysicalSize[uint32]) {
	if newSize.Width > 0 && newSize.Height > 0 {
		s.size = newSize
		s.config.Width = newSize.Width
		s.config.Height = newSize.Height

		if s.swapChain != nil {
			s.swapChain.Release()
		}
		var err error
		s.swapChain, err = s.device.CreateSwapChain(s.surface, s.config)
		if err != nil {
			panic(err)
		}
	}
}

func (s *State) Render() error {
	view, err := s.swapChain.GetCurrentTextureView()
	if err != nil {
		return err
	}
	defer view.Release()

	encoder, err := s.device.CreateCommandEncoder(nil)
	if err != nil {
		return err
	}
	defer encoder.Release()

	renderPass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:   view,
			LoadOp: wgpu.LoadOp_Clear,
			ClearValue: wgpu.Color{
				R: 0.1,
				G: 0.2,
				B: 0.3,
				A: 1.0,
			},
			StoreOp: wgpu.StoreOp_Store,
		}},
	})
	defer renderPass.Release()

	renderPass.SetPipeline(s.renderPipeline)
	renderPass.SetVertexBuffer(0, s.vertexBuffer, 0, wgpu.WholeSize)
	renderPass.SetIndexBuffer(s.indexBuffer, wgpu.IndexFormat_Uint16, 0, wgpu.WholeSize)
	renderPass.DrawIndexed(s.numIndices, 1, 0, 0, 0)
	renderPass.End()

	cmdBuffer, err := encoder.Finish(nil)
	if err != nil {
		return err
	}
	defer cmdBuffer.Release()

	s.queue.Submit(cmdBuffer)
	s.swapChain.Present()

	return nil
}

func (s *State) Destroy() {
	if s.indexBuffer != nil {
		s.indexBuffer.Release()
		s.indexBuffer = nil
	}
	if s.vertexBuffer != nil {
		s.vertexBuffer.Release()
		s.vertexBuffer = nil
	}
	if s.renderPipeline != nil {
		s.renderPipeline.Release()
		s.renderPipeline = nil
	}
	if s.swapChain != nil {
		s.swapChain.Release()
		s.swapChain = nil
	}
	if s.config != nil {
		s.config = nil
	}
	if s.queue != nil {
		s.queue.Release()
		s.queue = nil
	}
	if s.device != nil {
		s.device.Release()
		s.device = nil
	}
	if s.surface != nil {
		s.surface.Release()
		s.surface = nil
	}
}

func main() {
	d, err := display.NewDisplay()
	if err != nil {
		panic(err)
	}
	defer d.Destroy()

	w, err := display.NewWindow(d)
	if err != nil {
		panic(err)
	}
	defer w.Destroy()

	s, err := InitState(w)
	if err != nil {
		panic(err)
	}
	defer s.Destroy()

	w.SetResizedCallback(func(physicalWidth, physicalHeight uint32, scaleFactor float64) {
		s.Resize(dpi.PhysicalSize[uint32]{
			Width:  physicalWidth,
			Height: physicalHeight,
		})
	})

	w.SetCloseRequestedCallback(func() {
		d.Destroy()
	})

	for {
		if !d.Poll() {
			break
		}

		err := s.Render()
		if err != nil {
			fmt.Println("error occured while rendering:", err)

			errstr := err.Error()
			switch {
			case strings.Contains(errstr, "Surface timed out"): // do nothing
			case strings.Contains(errstr, "Surface is outdated"): // do nothing
			case strings.Contains(errstr, "Surface was lost"): // do nothing
			default:
				panic(err)
			}
		}
	}
}
