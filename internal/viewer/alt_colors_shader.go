package viewer

import (
	_ "embed"
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/jupiterrider/purego-sdl3/sdl"
)

// With SDL's GPU renderer, the alternate colours are drawn by a fragment
// shader: tiles stay as MuPDF rendered them, so switching colours renders
// nothing again. Other renderers fall back to remapping tiles on the render
// workers (remapPageColors).

//go:embed shaders/alt_colors.spv
var altColorsSPIRV []byte

// The GPU render state functions the SDL bindings leave out.
var (
	sdlCreateGPURenderState              func(*sdl.Renderer, *sdl.GPURenderStateCreateInfo) *sdl.GPURenderState
	sdlSetGPURenderStateFragmentUniforms func(*sdl.GPURenderState, uint32, unsafe.Pointer, uint32) bool
	sdlSetGPURenderState                 func(*sdl.Renderer, *sdl.GPURenderState) bool
	sdlDestroyGPURenderState             func(*sdl.GPURenderState)
	loadGPURenderState                   = sync.OnceValue(func() error {
		for name, fn := range map[string]any{
			"SDL_CreateGPURenderState":              &sdlCreateGPURenderState,
			"SDL_SetGPURenderStateFragmentUniforms": &sdlSetGPURenderStateFragmentUniforms,
			"SDL_SetGPURenderState":                 &sdlSetGPURenderState,
			"SDL_DestroyGPURenderState":             &sdlDestroyGPURenderState,
		} {
			sym, err := sdlSymbol(name)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			purego.RegisterFunc(fn, sym)
		}
		return nil
	})
)

type altColorsShader struct {
	device *sdl.GPUDevice
	shader *sdl.GPUShader
	state  *sdl.GPURenderState
}

// newAltColorsShader compiles the shader for renderer, failing when it is not
// SDL's GPU renderer or cannot run SPIR-V.
func newAltColorsShader(renderer *sdl.Renderer) (*altColorsShader, error) {
	device := sdl.GetGPURendererDevice(renderer)
	if device == nil {
		return nil, fmt.Errorf("not a GPU renderer")
	}
	if sdl.GetGPUShaderFormats(device)&sdl.GPUShaderFormatSPIRV == 0 {
		return nil, fmt.Errorf("GPU driver %s does not take SPIR-V", sdl.GetGPUDeviceDriver(device))
	}
	if err := loadGPURenderState(); err != nil {
		return nil, err
	}
	info := sdl.GPUShaderCreateInfo{
		CodeSize:          uint64(len(altColorsSPIRV)),
		Code:              &altColorsSPIRV[0],
		Format:            sdl.GPUShaderFormatSPIRV,
		Stage:             sdl.GPUShaderStageFragment,
		NumSamplers:       1,
		NumUniformBuffers: 1,
	}
	info.SetEntryPoint("main")
	shader := sdl.CreateGPUShader(device, &info)
	if shader == nil {
		return nil, fmt.Errorf("create shader: %s", sdl.GetError())
	}
	state := sdlCreateGPURenderState(renderer, &sdl.GPURenderStateCreateInfo{FragmentShader: shader})
	if state == nil {
		sdl.ReleaseGPUShader(device, shader)
		return nil, fmt.Errorf("create render state: %s", sdl.GetError())
	}
	return &altColorsShader{device: device, shader: shader, state: state}, nil
}

// begin makes renderer draw textures in the bg-fg range until end.
func (s *altColorsShader) begin(renderer *sdl.Renderer, bg, fg [3]uint8) {
	uniforms := [8]float32{
		float32(fg[0]) / 255, float32(fg[1]) / 255, float32(fg[2]) / 255, 1,
		float32(bg[0]) / 255, float32(bg[1]) / 255, float32(bg[2]) / 255, 1,
	}
	sdlSetGPURenderStateFragmentUniforms(s.state, 0, unsafe.Pointer(&uniforms[0]), uint32(unsafe.Sizeof(uniforms)))
	sdlSetGPURenderState(renderer, s.state)
}

func (s *altColorsShader) end(renderer *sdl.Renderer) {
	sdlSetGPURenderState(renderer, nil)
}

func (s *altColorsShader) Close() {
	if s == nil {
		return
	}
	sdlDestroyGPURenderState(s.state)
	sdl.ReleaseGPUShader(s.device, s.shader)
}
