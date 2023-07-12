package main

import (
	_ "embed"
	"fmt"
	"math"
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

//go:embed happy-tree.png
var happyTreePng []byte

const NumInstancesPerRow = 10
const RotationSpeedRad = 2.0 * math.Pi / 60.0

var InstanceDisplacement = glm.Vec3[float32]{
	NumInstancesPerRow * 0.5,
	0.0,
	NumInstancesPerRow * 0.5,
}

type Vertex struct {
	position  [3]float32
	texCoords [2]float32
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
			Format:         wgpu.VertexFormat_Float32x2,
		},
	},
}

var VERTICES = [...]Vertex{
	{
		position:  [3]float32{-0.0868241, -0.49240386, 0.0},
		texCoords: [2]float32{1.0 - 0.4131759, 1.0 - 0.00759614},
	}, // A
	{
		position:  [3]float32{-0.49513406, -0.06958647, 0.0},
		texCoords: [2]float32{1.0 - 0.0048659444, 1.0 - 0.43041354},
	}, // B
	{
		position:  [3]float32{-0.21918549, 0.44939706, 0.0},
		texCoords: [2]float32{1.0 - 0.28081453, 1.0 - 0.949397},
	}, // C
	{
		position:  [3]float32{0.35966998, 0.3473291, 0.0},
		texCoords: [2]float32{1.0 - 0.85967, 1.0 - 0.84732914},
	}, // D
	{
		position:  [3]float32{0.44147372, -0.2347359, 0.0},
		texCoords: [2]float32{1.0 - 0.9414737, 1.0 - 0.2652641},
	}, // E
}

var INDICES = [...]uint16{0, 1, 4, 1, 2, 4, 2, 3, 4}

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
		transform: glm.Mat4FromTranslation(i.position).Mul4(glm.Mat4FromQuaternion(i.rotation)),
	}
}

type InstanceRaw struct {
	transform glm.Mat4[float32]
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
			Offset:         uint64(unsafe.Sizeof([4]float32{})),
			ShaderLocation: 6,
			Format:         wgpu.VertexFormat_Float32x4,
		},
		{
			Offset:         uint64(unsafe.Sizeof([8]float32{})),
			ShaderLocation: 7,
			Format:         wgpu.VertexFormat_Float32x4,
		},
		{
			Offset:         uint64(unsafe.Sizeof([12]float32{})),
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
	vertexBuffer     *wgpu.Buffer
	indexBuffer      *wgpu.Buffer
	numIndices       uint32
	diffuseTexture   *Texture
	diffuseBindGroup *wgpu.BindGroup
	camera           *Camera
	cameraController *CameraController
	cameraUniform    *CameraUniform
	cameraBuffer     *wgpu.Buffer
	cameraBindGroup  *wgpu.BindGroup

	instances      [NumInstancesPerRow * NumInstancesPerRow]Instance
	instanceBuffer *wgpu.Buffer
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

	s.diffuseTexture, err = TextureFromPNGBytes(s.device, s.queue, happyTreePng, "happy-tree.png")
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
	defer textureBindGroupLayout.Release()

	s.diffuseBindGroup, err = s.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: textureBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding:     0,
				TextureView: s.diffuseTexture.view,
			},
			{
				Binding: 1,
				Sampler: s.diffuseTexture.sampler,
			},
		},
		Label: "DiffuseBindGroup",
	})
	if err != nil {
		return s, err
	}

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
		index := 0
		for z := 0; z < NumInstancesPerRow; z++ {
			for x := 0; x < NumInstancesPerRow; x++ {
				position := glm.Vec3[float32]{float32(x), 0, float32(z)}.Sub(InstanceDisplacement)

				var rotation glm.Quaternion[float32]
				if position == (glm.Vec3[float32]{}) {
					rotation = glm.QuaternionFromAxisAngle(glm.Vec3[float32]{0, 1, 0}, 0)
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
		Usage:    wgpu.BufferUsage_Vertex | wgpu.BufferUsage_CopyDst,
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
			},
		}},
	})
	if err != nil {
		return s, err
	}
	defer cameraBindGroupLayout.Release()

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
		BindGroupLayouts: []*wgpu.BindGroupLayout{
			textureBindGroupLayout, cameraBindGroupLayout,
		},
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
			Buffers:    []wgpu.VertexBufferLayout{VertexBufferLayout, InstanceBufferLayout},
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

var rotationAmount = glm.QuaternionFromAxisAngle(glm.Vec3[float32]{0, 1, 0}, RotationSpeedRad)

func (s *State) Update() {
	s.cameraController.UpdateCamera(s.camera)
	s.cameraUniform.UpdateViewProj(s.camera)
	s.queue.WriteBuffer(
		s.cameraBuffer,
		0,
		wgpu.ToBytes(s.cameraUniform.viewProj[:]),
	)

	var instanceData [NumInstancesPerRow * NumInstancesPerRow]InstanceRaw
	for i, v := range s.instances {
		v.rotation = rotationAmount.Mul(v.rotation)
		s.instances[i] = v
		instanceData[i] = v.ToRaw()
	}
	s.queue.WriteBuffer(
		s.instanceBuffer,
		0,
		wgpu.ToBytes(instanceData[:]),
	)
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

		s.camera.aspect = float32(newSize.Width) / float32(newSize.Height)
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
	renderPass.SetBindGroup(0, s.diffuseBindGroup, nil)
	renderPass.SetBindGroup(1, s.cameraBindGroup, nil)
	renderPass.SetVertexBuffer(0, s.vertexBuffer, 0, wgpu.WholeSize)
	renderPass.SetVertexBuffer(1, s.instanceBuffer, 0, wgpu.WholeSize)
	renderPass.SetIndexBuffer(s.indexBuffer, wgpu.IndexFormat_Uint16, 0, wgpu.WholeSize)
	renderPass.DrawIndexed(s.numIndices, uint32(len(s.instances)), 0, 0, 0)
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
	if s.cameraBindGroup != nil {
		s.cameraBindGroup.Release()
		s.cameraBindGroup = nil
	}
	if s.instanceBuffer != nil {
		s.instanceBuffer.Release()
		s.instanceBuffer = nil
	}
	if s.cameraBuffer != nil {
		s.cameraBuffer.Release()
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
	if s.diffuseBindGroup != nil {
		s.diffuseBindGroup.Release()
		s.diffuseBindGroup = nil
	}
	if s.diffuseTexture != nil {
		s.diffuseTexture.Destroy()
		s.diffuseTexture = nil
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

	w.SetKeyboardInputCallback(func(state events.ButtonState, scanCode events.ScanCode, virtualKeyCode events.VirtualKey) {
		isPressed := state == events.ButtonStatePressed

		switch virtualKeyCode {
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
