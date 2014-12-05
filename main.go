// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// An android app that draws a 3D gopher.
package main

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"log"
	"math"

	"compress/flate"

	"golang.org/x/mobile/app"
	"golang.org/x/mobile/app/debug"
	"golang.org/x/mobile/event"
	"golang.org/x/mobile/f32"
	"golang.org/x/mobile/geom"
	"golang.org/x/mobile/gl"
	"golang.org/x/mobile/gl/glutil"
)

//go:generate go run gengopher.go -output gopher.go -input gopher.obj

type piece struct {
	// flate compressed
	vertexData []byte
	normalData []byte
	color      f32.Vec4

	// populated at GL initialization
	verticies   gl.Buffer
	normals     gl.Buffer
	vertexCount int
}

var (
	gopherSkin = f32.Vec4{0.761, 0.442, 0.180, 1} // brownish
	gopherFur  = f32.Vec4{0, 0.537, 0.8, 1}       // blue
	white      = f32.Vec4{1, 1, 1, 1}
)

var pieces = []*piece{
	{
		vertexData: Body_Sphere_002,
		normalData: Body_Sphere_002Normals,
		color:      gopherFur,
	},
	{
		vertexData: Tail_Sphere_015,
		normalData: Tail_Sphere_015Normals,
		color:      gopherSkin,
	},
	{
		vertexData: Foot_R_001_Sphere_014,
		normalData: Foot_R_001_Sphere_014Normals,
		color:      gopherSkin,
	},
	{
		vertexData: Foot_R_Sphere_013,
		normalData: Foot_R_Sphere_013Normals,
		color:      gopherSkin,
	},
	{
		vertexData: Hnad_L_Sphere_012,
		normalData: Hnad_L_Sphere_012Normals,
		color:      gopherSkin,
	},
	{
		vertexData: Hand_R_Sphere_011,
		normalData: Hand_R_Sphere_011Normals,
		color:      gopherSkin,
	},
	{
		vertexData: Tooth_Sphere_009,
		normalData: Tooth_Sphere_009Normals,
		color:      white,
	},
	{
		vertexData: Ear_R_Sphere_008,
		normalData: Ear_R_Sphere_008Normals,
		color:      gopherFur,
	},
	{
		vertexData: Ear_L_Sphere_007,
		normalData: Ear_L_Sphere_007Normals,
		color:      gopherFur,
	},
	{
		vertexData: Nose_Sphere,
		normalData: Nose_SphereNormals,
		color:      gopherSkin,
	},
	{
		vertexData: Eye_R_Sphere_006,
		normalData: Eye_R_Sphere_006Normals,
		color:      white,
	},
	{
		vertexData: Eye_L_Sphere_004,
		normalData: Eye_L_Sphere_004Normals,
		color:      white,
	},
}

var (
	program gl.Program

	position gl.Attrib
	normal   gl.Attrib

	lightDirection        gl.Uniform
	lightAmbientColor     gl.Uniform
	lightDiffuseColor     gl.Uniform
	materialAmbientFactor gl.Uniform
	materialDiffuseFactor gl.Uniform
	materialShininess     gl.Uniform
	model                 gl.Uniform
	view                  gl.Uniform
	projection            gl.Uniform

	touchLoc geom.Point
)

func main() {
	app.Run(app.Callbacks{
		Draw:  draw,
		Touch: touch,
	})
}

func initGL() {
	gl.Enable(gl.DEPTH_TEST)
	//gl.Enable(gl.CULL_FACE)
	//gl.CullFace(gl.BACK)

	var err error
	program, err = glutil.CreateProgram(vertexShader, fragmentShader)
	if err != nil {
		log.Printf("error creating GL program: %v", err)
		return
	}

	for _, p := range pieces {
		vData := flateBytes(p.vertexData)
		nData := flateBytes(p.normalData)
		p.verticies = gl.GenBuffer()
		p.normals = gl.GenBuffer()
		// four bytes per float32, three per vertex
		p.vertexCount = len(vData) / 4 / coordsPerVertex

		gl.BindBuffer(gl.ARRAY_BUFFER, p.verticies)
		gl.BufferData(gl.ARRAY_BUFFER, gl.STATIC_DRAW, vData)
		gl.BindBuffer(gl.ARRAY_BUFFER, p.normals)
		gl.BufferData(gl.ARRAY_BUFFER, gl.STATIC_DRAW, nData)
	}

	position = gl.GetAttribLocation(program, "position")
	normal = gl.GetAttribLocation(program, "normal")

	lightDirection = gl.GetUniformLocation(program, "lightDirection")
	lightAmbientColor = gl.GetUniformLocation(program, "lightAmbientColor")
	lightDiffuseColor = gl.GetUniformLocation(program, "lightDiffuseColor")
	materialAmbientFactor = gl.GetUniformLocation(program, "materialAmbientFactor")
	materialDiffuseFactor = gl.GetUniformLocation(program, "materialDiffuseFactor")
	model = gl.GetUniformLocation(program, "model")
	view = gl.GetUniformLocation(program, "view")
	projection = gl.GetUniformLocation(program, "projection")

	initMVP()
}

func initMVP() {
	gl.UseProgram(program)
}

func touch(t event.Touch) {
	log.Printf("%s", t)
	touchLoc = t.Loc
}

func draw() {
	if program.Value == 0 {
		initGL()
		log.Printf("example/basic rendering initialized")
	}

	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.DEPTH_BUFFER_BIT | gl.COLOR_BUFFER_BIT)

	gl.UseProgram(program)

	frac := float32(touchLoc.X / geom.Width)
	y := 5 * f32.Sin(2*float32(math.Pi)*frac)
	z := 5 * f32.Cos(2*float32(math.Pi)*frac)

	mProj := f32.Mat4{}
	mProj.Perspective(f32.Radian(math.Pi/4), float32(geom.Width/geom.Height), .1, 200)
	projection.WriteMat4(&mProj)

	mView := f32.Mat4{}
	// Debugging note: pos 0,5,0 leaves you looking right at the gopher
	mView.LookAt(
		&f32.Vec3{0, y, -z}, // camera position
		&f32.Vec3{0, 0, 0},  // camera is pointing at
		&f32.Vec3{-1, 0, 0}) // rotation
	view.WriteMat4(&mView)

	// Gopher model starts on the origin.
	// Her up is -x, her forward is +z.
	mModel := f32.Mat4{}
	mModel.Identity()

	scale := float32(touchLoc.Y/geom.Height + 0.5)
	mModel.Scale(&mModel, scale, scale, scale)
	model.WriteMat4(&mModel)

	gl.Uniform3f(lightDirection, .5, .5, 0)
	gl.Uniform4f(materialDiffuseFactor, 0.8, 0.8, 0.8, 1)
	gl.Uniform4f(materialAmbientFactor, 0.5, 0.5, 0.5, 0.5)

	gl.EnableVertexAttribArray(normal)
	gl.EnableVertexAttribArray(position)
	for _, p := range pieces {
		lightDiffuseColor.WriteVec4(&p.color)
		lightAmbientColor.WriteVec4(&p.color)

		gl.BindBuffer(gl.ARRAY_BUFFER, p.verticies)
		gl.VertexAttribPointer(position, coordsPerVertex, gl.FLOAT, false, 0, 0)
		gl.BindBuffer(gl.ARRAY_BUFFER, p.normals)
		gl.VertexAttribPointer(normal, coordsPerVertex, gl.FLOAT, false, 0, 0)

		gl.DrawArrays(gl.TRIANGLES, 0, p.vertexCount)
	}
	gl.DisableVertexAttribArray(normal)
	gl.DisableVertexAttribArray(position)

	debug.DrawFPS()
}

func toBytes(v []float32) []byte {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, v); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

func flateBytes(v []byte) []byte {
	b, err := ioutil.ReadAll(flate.NewReader(bytes.NewReader(v)))
	if err != nil {
		log.Fatal(err)
	}
	return b
}

const coordsPerVertex = 3

const vertexShader = `
uniform vec3 lightDirection;
uniform vec4 lightAmbientColor;
uniform vec4 lightDiffuseColor;

uniform vec4 materialAmbientFactor;
uniform vec4 materialDiffuseFactor;

uniform mat4 model;
uniform mat4 view;
uniform mat4 projection;

attribute vec4 position;
attribute vec3 normal;

varying vec4 color;

void main() {
	mat4 mv = view * model;
	mat4 mvp = projection * mv;

	vec3 eyespace = vec3(mv * vec4(normal, 0.0));
	eyespace = eyespace / length(eyespace);

	float direction = max(0.0, dot(eyespace, lightDirection));

	vec4 ambient = lightAmbientColor * materialAmbientFactor;
	vec4 diffuse = direction * lightDiffuseColor * materialDiffuseFactor;

	color = ambient + diffuse;
	gl_Position = mvp * position;
}
`

const fragmentShader = `
precision mediump float;
varying vec4 color;
void main() {
	gl_FragColor = color;
}`
