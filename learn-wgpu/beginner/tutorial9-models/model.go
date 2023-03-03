package main

import (
	"unsafe"

	"github.com/rajveermalviya/go-webgpu/wgpu"
)

type ModelVertex struct {
	Position  [3]float32
	TexCoords [2]float32
	Normal    [3]float32
}

var ModelVertexLayout = wgpu.VertexBufferLayout{
	ArrayStride: uint64(unsafe.Sizeof(ModelVertex{})),
	StepMode:    wgpu.VertexStepMode_Vertex,
	Attributes: []wgpu.VertexAttribute{
		{
			Offset:         0,
			ShaderLocation: 0,
			Format:         wgpu.VertexFormat_Float32x3,
		},
		{
			Offset:         0 + wgpu.VertexFormat_Float32x3.Size(),
			ShaderLocation: 1,
			Format:         wgpu.VertexFormat_Float32x2,
		},
		{
			Offset:         0 + wgpu.VertexFormat_Float32x3.Size() + wgpu.VertexFormat_Float32x2.Size(),
			ShaderLocation: 2,
			Format:         wgpu.VertexFormat_Float32x3,
		},
	},
}

type Material struct {
	Name           string
	DiffuseTexture *Texture
	BindGroup      *wgpu.BindGroup
}

type Mesh struct {
	Name         string
	VertexBuffer *wgpu.Buffer
	IndexBuffer  *wgpu.Buffer
	NumElements  uint32
	MaterialIdx  int
}

type Model struct {
	Meshes    []Mesh
	Materials []Material
}

func (m *Model) Destroy() {
	for _, mesh := range m.Meshes {
		mesh.VertexBuffer.Drop()
		mesh.IndexBuffer.Drop()
	}
	m.Meshes = nil

	for _, mtl := range m.Materials {
		mtl.DiffuseTexture.Destroy()
		mtl.BindGroup.Drop()
	}
	m.Materials = nil
}

func drawModelInstanced(renderPass *wgpu.RenderPassEncoder, model *Model, cameraBindGroup *wgpu.BindGroup, instanceCount uint32) {
	for _, mesh := range model.Meshes {
		material := model.Materials[mesh.MaterialIdx]

		renderPass.SetVertexBuffer(0, mesh.VertexBuffer, 0, wgpu.WholeSize)
		renderPass.SetIndexBuffer(mesh.IndexBuffer, wgpu.IndexFormat_Uint32, 0, wgpu.WholeSize)
		renderPass.SetBindGroup(0, material.BindGroup, nil)
		renderPass.SetBindGroup(1, cameraBindGroup, nil)
		renderPass.DrawIndexed(mesh.NumElements, instanceCount, 0, 0, 0)
	}
}
