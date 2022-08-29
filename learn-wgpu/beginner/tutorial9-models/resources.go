package main

import (
	"embed"
	"errors"
	"io"

	"github.com/rajveermalviya/go-webgpu-examples/learn-wgpu/beginner/tutorial9-models/objloader"
	"github.com/rajveermalviya/go-webgpu/wgpu"
	"golang.org/x/exp/slices"
)

//go:embed res
var res embed.FS

func loadTexture(name string, device *wgpu.Device, queue *wgpu.Queue) (*Texture, error) {
	f, err := res.Open("res/" + name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return TextureFromBytes(device, queue, buf, name)
}

func LoadModel(device *wgpu.Device, queue *wgpu.Queue, layout *wgpu.BindGroupLayout) (*Model, error) {
	models, objMaterials, err := objloader.LoadObj(res, "res/cube.obj")
	if err != nil {
		return nil, err
	}

	materials := []Material{}
	for _, m := range objMaterials {
		diffuseTexture, err := loadTexture(m.DiffuseTexture, device, queue)
		if err != nil {
			return nil, err
		}

		bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Layout: layout,
			Entries: []wgpu.BindGroupEntry{
				{
					Binding:     0,
					TextureView: diffuseTexture.view,
				},
				{
					Binding: 1,
					Sampler: diffuseTexture.sampler,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		materials = append(materials, Material{
			Name:           m.Name,
			DiffuseTexture: diffuseTexture,
			BindGroup:      bindGroup,
		})
	}

	meshes := []Mesh{}

	for _, m := range models {
		if len(m.Normals) != len(m.TextureCoords) || len(m.TextureCoords) != len(m.Vertices) {
			return nil, errors.New("got invalid obj")
		}

		vertices := []ModelVertex{}
		for i := 0; i < len(m.Vertices); i++ {
			pos := m.Vertices[i]
			texCoords := m.TextureCoords[i]
			normal := m.Normals[i]

			vertices = append(vertices, ModelVertex{
				Position:  pos,
				TexCoords: [2]float32{texCoords[0], texCoords[1]},
				Normal:    normal,
			})
		}

		vertexBuffer, err := device.CreateBufferInit(&wgpu.BufferInitDescriptor{
			Label:    m.Name + " vertex buffer",
			Contents: wgpu.ToBytes(vertices),
			Usage:    wgpu.BufferUsage_Vertex,
		})
		if err != nil {
			return nil, err
		}

		indexBuffer, err := device.CreateBufferInit(&wgpu.BufferInitDescriptor{
			Label:    m.Name + " index buffer",
			Contents: wgpu.ToBytes(m.Indices),
			Usage:    wgpu.BufferUsage_Index,
		})
		if err != nil {
			return nil, err
		}

		materialIdx := slices.IndexFunc(materials,
			func(e Material) bool { return e.Name == m.MaterialName },
		)
		if materialIdx == -1 {
			materialIdx = 0
		}

		meshes = append(meshes, Mesh{
			Name:         m.Name,
			VertexBuffer: vertexBuffer,
			IndexBuffer:  indexBuffer,
			NumElements:  uint32(len(m.Indices)),
			MaterialIdx:  materialIdx,
		})
	}

	return &Model{
		Meshes:    meshes,
		Materials: materials,
	}, nil
}
