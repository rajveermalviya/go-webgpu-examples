package objloader

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

type Model struct {
	Name          string
	MaterialName  string
	Vertices      [][3]float32
	TextureCoords [][3]float32
	Normals       [][3]float32
	Indices       []uint32
}

type Material struct {
	Name              string
	Ambient           [3]float32
	Diffuse           [3]float32
	Specular          [3]float32
	Shininess         float32
	Dissolve          float32
	OpticalDensity    float32
	AmbientTexture    string
	DiffuseTexture    string
	SpecularTexture   string
	NormalTexture     string
	ShininessTexture  string
	DissolveTexture   string
	IlluminationModel uint8
}

func LoadObj(dir fs.FS, obj string) ([]Model, []Material, error) {
	f, err := dir.Open(obj)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var (
		models    []Model
		materials []Material
	)

	var (
		currentModel    string = "unnamed_object"
		currentMaterial string
		tmpVertices     [][3]float32
		tmpNormals      [][3]float32
		tmpTexCoords    [][3]float32
		tmpFaceElems    [][3][3]int64
		lineNumber      int
	)

	s := bufio.NewScanner(f)
	for s.Scan() {
		lineNumber++

		l := strings.TrimSpace(s.Text())
		split := strings.Split(l, " ")
		if len(split) < 1 {
			return nil, nil, fmt.Errorf("invalid tokens at line %d", lineNumber)
		}

		switch split[0] {
		case "o":
			if len(split) < 2 {
				return nil, nil, fmt.Errorf("invalid object name at line %d", lineNumber)
			}

			name := split[1]
			if name != currentModel && len(tmpVertices) != 0 {
				model := exportModel(
					currentModel,
					tmpVertices,
					tmpNormals,
					tmpTexCoords,
					tmpFaceElems,
				)
				model.MaterialName = currentMaterial
				models = append(models, model)
			}

			currentModel = name

		case "v":
			if len(split) < 4 {
				return nil, nil, fmt.Errorf("invalid vertex at line %d", lineNumber)
			}

			x, y, z, err := parse3Float(split[1], split[2], split[3])
			if err != nil {
				return nil, nil, fmt.Errorf("invalid vertex at line %d", lineNumber)
			}
			tmpVertices = append(tmpVertices, [3]float32{x, y, z})

		case "vn":
			if len(split) < 4 {
				return nil, nil, fmt.Errorf("invalid vertex normal at line %d", lineNumber)
			}

			i, j, k, err := parse3Float(split[1], split[2], split[3])
			if err != nil {
				return nil, nil, fmt.Errorf("invalid vertex normal at line %d", lineNumber)
			}
			tmpNormals = append(tmpNormals, [3]float32{i, j, k})

		case "vt":
			if len(split) < 2 {
				return nil, nil, fmt.Errorf("invalid texture coordinates at line %d", lineNumber)
			}

			u, err := strconv.ParseFloat(split[1], 32)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid texture coordinates at line %d", lineNumber)
			}
			v := float64(0)
			if len(split) >= 3 {
				v, err = strconv.ParseFloat(split[2], 32)
				if err != nil {
					return nil, nil, fmt.Errorf("invalid texture coordinates at line %d", lineNumber)
				}
			}
			w := float64(0)
			if len(split) >= 4 {
				w, err = strconv.ParseFloat(split[3], 32)
				if err != nil {
					return nil, nil, fmt.Errorf("invalid texture coordinates at line %d", lineNumber)
				}
			}

			tmpTexCoords = append(tmpTexCoords, [3]float32{float32(u), float32(v), float32(w)})

		case "f", "l":
			if len(split) < 2 {
				return nil, nil, fmt.Errorf("invalid face/line element at line %d", lineNumber)
			}

			switch len(split) - 1 {
			case 1: // point
				fallthrough
			case 2: // line
				fallthrough
			case 4: // quad
				fallthrough
			default: // polygon
				return nil, nil, fmt.Errorf("unsupported face/line element at line %d", lineNumber)

			case 3: // triangle
				var indicesArr [3][3]int64

				for i, indices := range split[1:] {
					indicesSplit := strings.SplitN(indices, "/", 3)
					if len(indicesSplit) != 3 {
						return nil, nil, fmt.Errorf("unsupported face/line element at line %d", lineNumber)
					}

					vIdx, err := strconv.ParseInt(indicesSplit[0], 10, 64)
					if err != nil {
						return nil, nil, fmt.Errorf("invalid face/line element at line %d", lineNumber)
					}
					vtIdx, err := strconv.ParseInt(indicesSplit[1], 10, 64)
					if err != nil {
						return nil, nil, fmt.Errorf("invalid face/line element at line %d", lineNumber)
					}
					vnIdx, err := strconv.ParseInt(indicesSplit[2], 10, 64)
					if err != nil {
						return nil, nil, fmt.Errorf("invalid face/line element at line %d", lineNumber)
					}

					indicesArr[i] = [3]int64{vIdx, vtIdx, vnIdx}
				}

				tmpFaceElems = append(tmpFaceElems, indicesArr)
			}

		case "mtllib":
			if len(split) < 2 {
				return nil, nil, fmt.Errorf("invalid external .mtl reference at line %d", lineNumber)
			}

			mtl := split[1]
			mtls, err := loadMtl(dir, obj, mtl)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to load mtl file at line %d: %w", lineNumber, err)
			}

			materials = append(materials, mtls...)

		case "usemtl":
			if len(split) < 2 {
				return nil, nil, fmt.Errorf("invalid mtl reference at line %d", lineNumber)
			}

			currentMaterial = split[1]
		}
	}

	model := exportModel(
		currentModel,
		tmpVertices,
		tmpNormals,
		tmpTexCoords,
		tmpFaceElems,
	)
	models = append(models, model)

	return models, materials, nil
}

func parseFloat(s string, bitSize int) (float32, error) {
	r, err := strconv.ParseFloat(s, bitSize)
	return float32(r), err
}

func parse3Float(xs, ys, zs string) (x float32, y float32, z float32, err error) {
	x, err = parseFloat(xs, 32)
	if err != nil {
		return 0, 0, 0, err
	}
	y, err = parseFloat(ys, 32)
	if err != nil {
		return 0, 0, 0, err
	}
	z, err = parseFloat(zs, 32)
	if err != nil {
		return 0, 0, 0, err
	}
	return
}

func exportModel(name string, verts [][3]float32, normals [][3]float32, texCoords [][3]float32, faces [][3][3]int64) (model Model) {
	model.Name = name

	indexMap := map[[3]int64]uint32{}

	for _, face := range faces {
		for _, idx := range face {
			vIdx := idx[0] - 1
			vtIdx := idx[1] - 1
			vnIdx := idx[2] - 1

			if i, ok := indexMap[idx]; ok {
				model.Indices = append(model.Indices, i)
				continue
			} else {
				model.Vertices = append(model.Vertices, verts[vIdx])
				model.Normals = append(model.Normals, normals[vnIdx])
				model.TextureCoords = append(model.TextureCoords, texCoords[vtIdx])
				next := len(indexMap)
				model.Indices = append(model.Indices, uint32(next))
				indexMap[idx] = uint32(next)
			}
		}
	}
	return
}

func loadMtl(dir fs.FS, obj string, mtl string) ([]Material, error) {
	var mtlFile string
	if _, ok := dir.(embed.FS); ok {
		mtlFile = filepath.Dir(obj) + "/" + mtl
	} else {
		mtlFile = filepath.Join(filepath.Dir(obj), mtl)
	}

	f, err := dir.Open(mtlFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var materials []Material

	var (
		currentMtl = Material{Name: "unnamed_mtl"}
		lineNumber int
	)

	s := bufio.NewScanner(f)
	for s.Scan() {
		lineNumber++

		l := strings.TrimSpace(s.Text())
		split := strings.Split(l, " ")
		if len(split) < 1 {
			return nil, fmt.Errorf("invalid tokens at line %d", lineNumber)
		}

		switch split[0] {
		case "newmtl":
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid material name at line %d", lineNumber)
			}

			name := split[1]
			if name != currentMtl.Name {
				zero := Material{Name: "unnamed_mtl"}
				if currentMtl != zero {
					materials = append(materials, currentMtl)
				}

				currentMtl = Material{Name: name}
			}

		case "Ka": // ambient
			if len(split) < 4 {
				return nil, fmt.Errorf("invalid Ka at line %d", lineNumber)
			}

			x, y, z, err := parse3Float(split[1], split[2], split[3])
			if err != nil {
				return nil, fmt.Errorf("invalid Ka at line %d", lineNumber)
			}

			currentMtl.Ambient = [3]float32{x, y, z}

		case "Kd": // diffuse
			if len(split) < 4 {
				return nil, fmt.Errorf("invalid Kd at line %d", lineNumber)
			}

			x, y, z, err := parse3Float(split[1], split[2], split[3])
			if err != nil {
				return nil, fmt.Errorf("invalid Kd at line %d", lineNumber)
			}

			currentMtl.Diffuse = [3]float32{x, y, z}

		case "Ks": // specular
			if len(split) < 4 {
				return nil, fmt.Errorf("invalid Ks at line %d", lineNumber)
			}

			x, y, z, err := parse3Float(split[1], split[2], split[3])
			if err != nil {
				return nil, fmt.Errorf("invalid Ks at line %d", lineNumber)
			}

			currentMtl.Specular = [3]float32{x, y, z}

		case "Ns": // shininess
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid Ns at line %d", lineNumber)
			}

			x, err := parseFloat(split[1], 32)
			if err != nil {
				return nil, fmt.Errorf("invalid Ns at line %d", lineNumber)
			}

			currentMtl.Shininess = x

		case "Ni": // optical_density
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid Ni at line %d", lineNumber)
			}

			x, err := parseFloat(split[1], 32)
			if err != nil {
				return nil, fmt.Errorf("invalid Ni at line %d", lineNumber)
			}

			currentMtl.OpticalDensity = x

		case "d": // dissolve
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid d at line %d", lineNumber)
			}

			x, err := parseFloat(split[1], 32)
			if err != nil {
				return nil, fmt.Errorf("invalid d at line %d", lineNumber)
			}

			currentMtl.Dissolve = x

		case "map_Ka": // ambient_texture
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid map_Ka at line %d", lineNumber)
			}

			currentMtl.AmbientTexture = split[1]

		case "map_Kd": // diffuse_texture
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid map_Kd at line %d", lineNumber)
			}

			currentMtl.DiffuseTexture = split[1]

		case "map_Ks": // specular_texture
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid map_Ks at line %d", lineNumber)
			}

			currentMtl.SpecularTexture = split[1]

		case "map_Bump", "map_bump": // normal_texture
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid map_Bump at line %d", lineNumber)
			}

			currentMtl.NormalTexture = split[1]

		case "map_Ns", "map_ns", "map_NS": // shininess_texture
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid map_Ns at line %d", lineNumber)
			}

			currentMtl.ShininessTexture = split[1]

		case "bump": // normal_texture
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid bump at line %d", lineNumber)
			}

			currentMtl.NormalTexture = split[1]

		case "map_d": // dissolve_texture
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid map_d at line %d", lineNumber)
			}

			currentMtl.DissolveTexture = split[1]

		case "illum": // illumination_model
			if len(split) < 2 {
				return nil, fmt.Errorf("invalid illum at line %d", lineNumber)
			}

			x, err := strconv.ParseUint(split[1], 10, 8)
			if err != nil {
				return nil, fmt.Errorf("invalid illum at line %d", lineNumber)
			}

			currentMtl.IlluminationModel = uint8(x)

		case "#", "": // comment (ignored)
		default: // unknown
		}
	}

	materials = append(materials, currentMtl)

	return materials, nil
}
