// Maps each pixel's luminance onto the fg-bg range, as remapPageColors does
// on the CPU. The inputs and bindings are those SDL's GPU renderer gives a
// custom fragment shader.
//
// Rebuild alt_colors.spv with:
//   glslc -fshader-stage=frag alt_colors.frag -o alt_colors.spv
#version 450

layout(location = 0) in vec4 v_color;
layout(location = 1) in vec2 v_uv;
layout(set = 2, binding = 0) uniform sampler2D u_texture;
layout(set = 3, binding = 0) uniform AltColors {
	vec4 fg;
	vec4 bg;
};
layout(location = 0) out vec4 o_color;

void main() {
	vec4 c = texture(u_texture, v_uv);
	float t = dot(c.rgb, vec3(77.0, 150.0, 29.0) / 256.0);
	o_color = vec4(mix(fg.rgb, bg.rgb, t), c.a) * v_color;
}
