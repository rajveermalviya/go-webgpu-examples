package main

import (
	_ "embed"
	"fmt"
	"strings"
	"unsafe"

	"github.com/rajveermalviya/gamen/display"
	"github.com/rajveermalviya/gamen/dpi"
	"github.com/rajveermalviya/gamen/events"
	"github.com/rajveermalviya/go-webgpu-examples/internal/glm"
	"github.com/rajveermalviya/go-webgpu/wgpu"
)

//go:embed shader.wgsl
var shaderCode string

const NumInstancesPerRow = 10

var OpenGlToWgpuMatrix = glm.Mat4[float32]{
	1.0, 0.0, 0.0, 0.0,
	0.0, 1.0, 0.0, 0.0,
	0.0, 0.0, 0.5, 0.0,
	0.0, 0.0, 0.5, 1.0,
}

type Camera struct {
	eye     glm.Vec3[float32]
	target  glm.Vec3[float32]
	up      glm.Vec3[float32]
	aspect  float32
	fovYRad float32
	znear   float32
	zfar    float32
}

func (c *Camera) buildViewProjectionMatrix() glm.Mat4[float32] {
	view := glm.LookAtRH(c.eye, c.target, c.up)
	proj := glm.Perspective(c.fovYRad, c.aspect, c.znear, c.zfar)
	return proj.Mul4(view)
}

type CameraUniform struct {
	viewProj glm.Mat4[float32]
}

func NewCameraUnifrom() *CameraUniform {
	return &CameraUniform{
		viewProj: glm.Mat4[float32]{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, 1, 0,
			0, 0, 0, 1,
		},
	}
}

func (c *CameraUniform) UpdateViewProj(camera *Camera) {
	c.viewProj = OpenGlToWgpuMatrix.Mul4(camera.buildViewProjectionMatrix())
}

type CameraController struct {
	speed             float32
	isUpPressed       bool
	isDownPressed     bool
	isForwardPressed  bool
	isBackwardPressed bool
	isLeftPressed     bool
	isRightPressed    bool
}

func NewCameraController(speed float32) *CameraController {
	return &CameraController{speed: speed}
}

func (c *CameraController) UpdateCamera(camera *Camera) {
	forward := camera.target.Sub(camera.eye)
	forwardNorm := forward.Normalize()
	forwardMag := forward.Magnitude()

	if c.isForwardPressed && forwardMag > c.speed {
		camera.eye = camera.eye.Add(forwardNorm.MulScalar(c.speed))
	}
	if c.isBackwardPressed {
		camera.eye = camera.eye.Sub(forwardNorm.MulScalar(c.speed))
	}

	right := forwardNorm.Cross(camera.up)

	forward = camera.target.Sub(camera.eye)
	forwardMag = forward.Magnitude()

	if c.isRightPressed {
		camera.eye = camera.target.Sub(forward.Add(right.MulScalar(c.speed)).Normalize().MulScalar(forwardMag))
	}
	if c.isLeftPressed {
		camera.eye = camera.target.Sub(forward.Sub(right.MulScalar(c.speed)).Normalize().MulScalar(forwardMag))
	}
}

type Instance struct {
	position glm.Vec3[float32]
	rotation glm.Quaternion[float32]
}

func (i Instance) ToRaw() InstanceRaw {
	return InstanceRaw{
		model: glm.Mat4FromTranslation(i.position).Mul4(glm.Mat4FromQuaternion(i.rotation)),
	}
}

type InstanceRaw struct {
	model glm.Mat4[float32]
}

var InstanceBufferLayout = wgpu.VertexBufferLayout{
	ArrayStride: uint64(unsafe.Sizeof(InstanceRaw{})),
	StepMode:    wgpu.VertexStepMode_Instance,
	Attributes: []wgpu.VertexAttribute{
		{
			Offset:         0,
			ShaderLocation: 5,
			Format:         wgpu.VertexFormat_Float32x4,
		},
		{
			Offset:         wgpu.VertexFormat_Float32x4.Size(),
			ShaderLocation: 6,
			Format:         wgpu.VertexFormat_Float32x4,
		},
		{
			Offset:         wgpu.VertexFormat_Float32x4.Size() * 2,
			ShaderLocation: 7,
			Format:         wgpu.VertexFormat_Float32x4,
		},
		{
			Offset:         wgpu.VertexFormat_Float32x4.Size() * 3,
			ShaderLocation: 8,
			Format:         wgpu.VertexFormat_Float32x4,
		},
	},
}

type State struct {
	surface          *wgpu.Surface
	swapChain        *wgpu.SwapChain
	device           *wgpu.Device
	queue            *wgpu.Queue
	config           *wgpu.SwapChainDescriptor
	size             dpi.PhysicalSize[uint32]
	renderPipeline   *wgpu.RenderPipeline
	objModel         *Model
	camera           *Camera
	cameraController *CameraController
	cameraUniform    *CameraUniform
	cameraBuffer     *wgpu.Buffer
	cameraBindGroup  *wgpu.BindGroup
	instances        [NumInstancesPerRow * NumInstancesPerRow]Instance
	instanceBuffer   *wgpu.Buffer
	depthTexture     *Texture
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
	defer instance.Drop()

	s.surface = instance.CreateSurface(getSurfaceDescriptor(window))

	adaper, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		CompatibleSurface: s.surface,
	})
	if err != nil {
		return s, err
	}
	defer adaper.Drop()

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

	textureBindGroupLayout, err := s.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStage_Fragment,
				Texture: wgpu.TextureBindingLayout{
					Multisampled:  false,
					ViewDimension: wgpu.TextureViewDimension_2D,
					SampleType:    wgpu.TextureSampleType_Float,
				},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStage_Fragment,
				Sampler: wgpu.SamplerBindingLayout{
					Type: wgpu.SamplerBindingType_Filtering,
				},
			},
		},
		Label: "TextureBindGroupLayout",
	})
	if err != nil {
		return s, err
	}
	defer textureBindGroupLayout.Drop()

	s.camera = &Camera{
		eye:     glm.Vec3[float32]{0, 5, -10},
		target:  glm.Vec3[float32]{0, 0, 0},
		up:      glm.Vec3[float32]{0, 1, 0},
		aspect:  float32(s.size.Width) / float32(s.size.Height),
		fovYRad: glm.DegToRad[float32](45),
		znear:   0.1,
		zfar:    100.0,
	}
	s.cameraController = NewCameraController(0.2)
	s.cameraUniform = NewCameraUnifrom()
	s.cameraUniform.UpdateViewProj(s.camera)

	s.cameraBuffer, err = s.device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "Camera Buffer",
		Contents: wgpu.ToBytes(s.cameraUniform.viewProj[:]),
		Usage:    wgpu.BufferUsage_Uniform | wgpu.BufferUsage_CopyDst,
	})
	if err != nil {
		return s, err
	}

	s.instances = [NumInstancesPerRow * NumInstancesPerRow]Instance{}
	{
		const SpaceBetween = 3.0

		index := 0
		for z := 0; z < NumInstancesPerRow; z++ {
			for x := 0; x < NumInstancesPerRow; x++ {
				x := SpaceBetween * (float32(x) - NumInstancesPerRow/2.0)
				z := SpaceBetween * (float32(z) - NumInstancesPerRow/2.0)

				position := glm.Vec3[float32]{x, 0.0, z}

				var rotation glm.Quaternion[float32]
				if position == (glm.Vec3[float32]{}) {
					rotation = glm.QuaternionFromAxisAngle(glm.Vec3[float32]{0, 0, 1}, 0)
				} else {
					rotation = glm.QuaternionFromAxisAngle(position.Normalize(), glm.DegToRad[float32](45))
				}

				s.instances[index] = Instance{
					position: position,
					rotation: rotation,
				}
				index++
			}
		}
	}

	var instanceData [NumInstancesPerRow * NumInstancesPerRow]InstanceRaw
	for i, v := range s.instances {
		instanceData[i] = v.ToRaw()
	}
	s.instanceBuffer, err = s.device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "Instance Buffer",
		Contents: wgpu.ToBytes(instanceData[:]),
		Usage:    wgpu.BufferUsage_Vertex,
	})
	if err != nil {
		return s, err
	}

	cameraBindGroupLayout, err := s.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "CameraBindGroupLayout",
		Entries: []wgpu.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: wgpu.ShaderStage_Vertex,
			Buffer: wgpu.BufferBindingLayout{
				Type:             wgpu.BufferBindingType_Uniform,
				HasDynamicOffset: false,
				MinBindingSize:   wgpu.WholeSize,
			},
		}},
	})
	if err != nil {
		return s, err
	}
	defer cameraBindGroupLayout.Drop()

	s.cameraBindGroup, err = s.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "CameraBindGroup",
		Layout: cameraBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{{
			Binding: 0,
			Buffer:  s.cameraBuffer,
			Size:    wgpu.WholeSize,
		}},
	})
	if err != nil {
		return s, err
	}

	s.objModel, err = LoadModel(s.device, s.queue, textureBindGroupLayout)
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
	defer shader.Drop()

	s.depthTexture, err = CreateDepthTexture(s.device, s.config, "DepthTexture")
	if err != nil {
		return s, err
	}

	renderPipelineLayout, err := s.device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label: "Render Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{
			textureBindGroupLayout, cameraBindGroupLayout,
		},
	})
	if err != nil {
		return s, err
	}
	defer renderPipelineLayout.Drop()

	s.renderPipeline, err = s.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "Render Pipeline",
		Layout: renderPipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     shader,
			EntryPoint: "vs_main",
			Buffers:    []wgpu.VertexBufferLayout{ModelVertexLayout, InstanceBufferLayout},
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
		DepthStencil: &wgpu.DepthStencilState{
			Format:            DepthTextureFormat,
			DepthWriteEnabled: true,
			DepthCompare:      wgpu.CompareFunction_Less,
			StencilFront: wgpu.StencilFaceState{
				Compare: wgpu.CompareFunction_Always,
			},
			StencilBack: wgpu.StencilFaceState{
				Compare: wgpu.CompareFunction_Always,
			},
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

	return s, nil
}

func (s *State) Update() {
	s.cameraController.UpdateCamera(s.camera)
	s.cameraUniform.UpdateViewProj(s.camera)
	s.queue.WriteBuffer(s.cameraBuffer, 0, wgpu.ToBytes(s.cameraUniform.viewProj[:]))
}

func (s *State) Resize(newSize dpi.PhysicalSize[uint32]) {
	if newSize.Width > 0 && newSize.Height > 0 {
		s.size = newSize
		s.config.Width = newSize.Width
		s.config.Height = newSize.Height

		if s.swapChain != nil {
			s.swapChain.Drop()
		}
		var err error
		s.swapChain, err = s.device.CreateSwapChain(s.surface, s.config)
		if err != nil {
			panic(err)
		}

		s.camera.aspect = float32(newSize.Width) / float32(newSize.Height)

		s.depthTexture.Destroy()
		s.depthTexture = nil
		s.depthTexture, err = CreateDepthTexture(s.device, s.config, "DepthTexture")
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
	defer view.Drop()

	encoder, err := s.device.CreateCommandEncoder(nil)
	if err != nil {
		return err
	}

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
		DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{
			View:              s.depthTexture.view,
			DepthClearValue:   1,
			DepthLoadOp:       wgpu.LoadOp_Clear,
			DepthStoreOp:      wgpu.StoreOp_Store,
			DepthReadOnly:     false,
			StencilClearValue: 0,
			StencilLoadOp:     wgpu.LoadOp_Load,
			StencilStoreOp:    wgpu.StoreOp_Store,
			StencilReadOnly:   true,
		},
	})

	renderPass.SetVertexBuffer(1, s.instanceBuffer, 0, wgpu.WholeSize)
	renderPass.SetPipeline(s.renderPipeline)
	drawModelInstanced(renderPass, s.objModel, s.cameraBindGroup, uint32(len(s.instances)))
	renderPass.End()

	s.queue.Submit(encoder.Finish(nil))
	s.swapChain.Present()

	return nil
}

func (s *State) Destroy() {
	if s.renderPipeline != nil {
		s.renderPipeline.Drop()
		s.renderPipeline = nil
	}
	if s.depthTexture != nil {
		s.depthTexture.Destroy()
		s.depthTexture = nil
	}
	if s.objModel != nil {
		s.objModel.Destroy()
		s.objModel = nil
	}
	if s.cameraBindGroup != nil {
		s.cameraBindGroup.Drop()
		s.cameraBindGroup = nil
	}
	if s.instanceBuffer != nil {
		s.instanceBuffer.Drop()
		s.instanceBuffer = nil
	}
	if s.cameraBuffer != nil {
		s.cameraBuffer.Drop()
		s.cameraBuffer = nil
	}
	if s.cameraUniform != nil {
		s.cameraUniform = nil
	}
	if s.cameraController != nil {
		s.cameraController = nil
	}
	if s.camera != nil {
		s.camera = nil
	}
	if s.swapChain != nil {
		s.swapChain.Drop()
		s.swapChain = nil
	}
	if s.config != nil {
		s.config = nil
	}
	if s.queue != nil {
		s.queue = nil
	}
	if s.device != nil {
		s.device.Drop()
		s.device = nil
	}
	if s.surface != nil {
		s.surface.Drop()
		s.surface = nil
	}
}

func main() {
	wgpu.SetLogLevel(wgpu.LogLevel_Trace)
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

	w.SetKeyboardInputCallback(func(state events.ButtonState, scanCode events.ScanCode, virtualKeyCode events.VirtualKey) {
		isPressed := state == events.ButtonStatePressed

		switch virtualKeyCode {
		case events.VirtualKeySpace:
			s.cameraController.isUpPressed = isPressed
		case events.VirtualKeyLShift:
			s.cameraController.isDownPressed = isPressed
		case events.VirtualKeyW, events.VirtualKeyUp:
			s.cameraController.isForwardPressed = isPressed
		case events.VirtualKeyA, events.VirtualKeyLeft:
			s.cameraController.isLeftPressed = isPressed
		case events.VirtualKeyS, events.VirtualKeyDown:
			s.cameraController.isBackwardPressed = isPressed
		case events.VirtualKeyD, events.VirtualKeyRight:
			s.cameraController.isRightPressed = isPressed
		}
	})

	w.SetCloseRequestedCallback(func() {
		d.Destroy()
	})

	for {
		if !d.Poll() {
			break
		}

		s.Update()
		err := s.Render()
		if err != nil {
			errstr := err.Error()
			fmt.Println(errstr)

			switch {
			case strings.Contains(errstr, "Lost"):
				s.Resize(s.size)
			case strings.Contains(errstr, "Outdated"):
				s.Resize(s.size)
			case strings.Contains(errstr, "Timeout"):
			default:
				panic(err)
			}
		}
	}
}
