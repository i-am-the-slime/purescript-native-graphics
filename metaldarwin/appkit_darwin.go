//go:build darwin && !js

package metaldarwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework Metal -framework QuartzCore

#import <Cocoa/Cocoa.h>
#import <Metal/Metal.h>
#import <QuartzCore/QuartzCore.h>
#include <stdlib.h>

extern void goRenderTick(void);

static NSWindow      *g_window  = nil;
static NSView        *g_view    = nil;
static CAMetalLayer  *g_layer   = nil;
static id<MTLDevice>       g_device   = nil;
static id<MTLCommandQueue> g_queue    = nil;
static id<MTLRenderPipelineState> g_pso       = nil;
static id<MTLRenderPipelineState> g_pso_diff  = nil;
static id<MTLRenderPipelineState> g_pso_tex   = nil;
static id<MTLRenderPipelineState> g_pso_msdf  = nil;
static id<MTLRenderPipelineState> g_pso_rrect = nil;
static id<MTLRenderPipelineState> g_pso_bgdots = nil;
static id<MTLRenderPipelineState> g_pso_lattice = nil;
static id<MTLBuffer>       g_vbuf     = nil;
static NSUInteger          g_vbuf_cap = 0;
static NSUInteger          g_vbuf_off = 0;
static id<MTLBuffer>       g_tbuf     = nil;
static NSUInteger          g_tbuf_cap = 0;
static NSUInteger          g_tbuf_off = 0;
static id<MTLTexture>      g_msaa_tex      = nil;
static id<MTLTexture>      g_stencil_tex   = nil;
static id<MTLTexture>      g_atlas         = nil;

// Indexed offscreen targets. The caller supplies layer count and composition.
// The maximum follows Metal's fragment texture argument limit.
#define MG_MAX_LAYERS 31
static int g_layer_count = 1;
#define MG_LAYER_COUNT g_layer_count
static int g_layer_global[MG_MAX_LAYERS] = {0};
static int g_composite[256 * 4] = {0};
static int g_composite_count = 0;
static id<MTLTexture>  g_layer_msaa[MG_MAX_LAYERS]    = {0};
static id<MTLTexture>  g_layer_resolve[MG_MAX_LAYERS] = {0};
static id<MTLTexture>  g_layer_stencil[MG_MAX_LAYERS] = {0};
static BOOL            g_layer_visited[MG_MAX_LAYERS] = {0};
static int             g_current_layer = -1;
static id<MTLRenderPipelineState> g_pso_compose = nil;
static id<MTLSamplerState> g_sampler       = nil;

// Blur capture uses a parallel set of targets, composed using the caller's
// recipe before separable Gaussian filtering and source-over restoration.
static BOOL            g_blur_capture = NO;
static id<MTLTexture>  g_blur_msaa[MG_MAX_LAYERS]    = {0};
static id<MTLTexture>  g_blur_resolve[MG_MAX_LAYERS] = {0};
static id<MTLTexture>  g_blur_stencil[MG_MAX_LAYERS] = {0};
static BOOL            g_blur_visited[MG_MAX_LAYERS] = {0};
static id<MTLTexture>  g_blur_scratchA = nil;
static id<MTLTexture>  g_blur_scratchB = nil;
static int             g_blur_prev_layer = 0;
static id<MTLRenderPipelineState> g_pso_gauss = nil;
static id<MTLRenderPipelineState> g_pso_blit  = nil;
static id<MTLRenderPipelineState> g_pso_clip = nil;
#define MG_CLIP_DEPTH 8
static id<MTLDepthStencilState> g_dss_none = nil;
static id<MTLDepthStencilState> g_dss_clip_fail = nil;
static id<MTLDepthStencilState> g_dss_clip_set[MG_CLIP_DEPTH] = {0};
static id<MTLDepthStencilState> g_dss_clip_toggle[MG_CLIP_DEPTH] = {0};
static id<MTLDepthStencilState> g_dss_clip_clear[MG_CLIP_DEPTH] = {0};
static id<MTLDepthStencilState> g_dss_clip_test[MG_CLIP_DEPTH] = {0};
static NSUInteger          g_sample_count = 8;
static NSUInteger g_requested_samples = 0;
static const MTLPixelFormat g_stencil_fmt  = MTLPixelFormatStencil8;

static inline BOOL mg_multisampling_enabled(void) {
    return g_sample_count > 1;
}

static NSUInteger mg_choose_sample_count(id<MTLDevice> device) {
    const NSUInteger candidates[] = { 8, 4, 2 };
    for (NSUInteger i = 0; i < sizeof(candidates) / sizeof(candidates[0]); i++) {
        if ([device supportsTextureSampleCount:candidates[i]]) return candidates[i];
    }
    return 1;
}

static NSUInteger mg_configured_sample_count(id<MTLDevice> device) {
    if (g_requested_samples > 0 && [device supportsTextureSampleCount:g_requested_samples])
        return g_requested_samples;
    return mg_choose_sample_count(device);
}

static void mg_configure_resolved_color(MTLRenderPassColorAttachmentDescriptor *ca,
                                        id<MTLTexture> target,
                                        id<MTLTexture> resolve,
                                        MTLStoreAction msaaStoreAction) {
    if (mg_multisampling_enabled()) {
        ca.texture        = target;
        ca.resolveTexture = resolve;
        ca.storeAction    = msaaStoreAction;
    } else {
        ca.texture        = resolve;
        ca.resolveTexture = nil;
        ca.storeAction    = MTLStoreActionStore;
    }
}

static id<CAMetalDrawable>          g_frame_drawable = nil;
static id<MTLCommandBuffer>         g_frame_cb       = nil;
static id<MTLRenderCommandEncoder>  g_frame_enc      = nil;
static float                        g_vp_w = 0, g_vp_h = 0;

// Input state indexed by the native virtual keycode.
static float g_mouse_x = 0, g_mouse_y = 0;
static int   g_mouse_down = 0;
static unsigned char g_key_down[256] = {0};
static id g_keydown_monitor = nil, g_keyup_monitor = nil, g_mouse_monitor = nil;
static inline void mg_set_key(unsigned short code, int down) {
    if (code < 256) g_key_down[code] = (unsigned char)(down ? 1 : 0);
}
static float mg_get_mouse_x(void)  { return g_mouse_x; }
static float mg_get_mouse_y(void)  { return g_mouse_y; }
static int   mg_get_mouse_down(void) { return g_mouse_down; }
static int   mg_get_key(int code)  { return (code >= 0 && code < 256) ? (int)g_key_down[code] : 0; }

// MgTickTarget interface declared here so the preamble can reference it;
// the @implementation lives in tick_target_darwin.m so it isn't emitted into
// every cgo-generated .c (which would dup the Obj-C class symbol).
@interface MgTickTarget : NSObject
- (void)tick:(id)sender;
@end
static MgTickTarget *g_tickTarget = nil;
// g_tickTimer and mg_stop_tick live in tick_stop_darwin.m: cgo splices this
// preamble into multiple generated translation units, so a non-static
// definition here would emit two _mg_stop_tick symbols and fail to link.
extern NSTimer *g_tickTimer;
extern void mg_stop_tick(void);

// Native close requests return control to the host after stopping callbacks.
@interface MgWinDelegate : NSObject <NSWindowDelegate>
@end
static MgWinDelegate *g_winDelegate = nil;

@interface MgAppDelegate : NSObject <NSApplicationDelegate>
@end
static MgAppDelegate *g_appDelegate = nil;

// Caller-positioned region excluded from background window dragging.
@interface MgNoDragView : NSView
@end
static MgNoDragView *g_noDragView = nil;

static void mg_install_menu(const char *title, const char *key) {
    if (title == NULL || title[0] == '\0') return;
    NSMenu *menubar = [[NSMenu alloc] init];
    NSMenuItem *appItem = [[NSMenuItem alloc] init];
    [menubar addItem:appItem];
    NSMenu *menu = [[NSMenu alloc] init];
    [menu addItemWithTitle:[NSString stringWithUTF8String:title]
                   action:@selector(terminate:)
            keyEquivalent:[NSString stringWithUTF8String:key]];
    [appItem setSubmenu:menu];
    [NSApp setMainMenu:menubar];
}

static void mg_set_icon(const unsigned char *bytes, int length) {
    if (length <= 0) return;
    NSData *data = [NSData dataWithBytes:bytes length:(NSUInteger)length];
    NSImage *image = [[NSImage alloc] initWithData:data];
    if (image != nil) [NSApp setApplicationIconImage:image];
}

static void mg_set_title(const char *title) {
    [g_window setTitle:[NSString stringWithUTF8String:title]];
}

static void mg_set_drag_exclusion(double x, double y, double w, double h) {
    CGFloat height = [[g_window contentView] bounds].size.height;
    [g_noDragView setFrame:NSMakeRect(x, height-y-h, MAX(0,w), MAX(0,h))];
    [g_noDragView setHidden:w <= 0 || h <= 0];
}

extern int mg_quit_requested;
extern void mg_request_stop(void);

static int mg_get_quit_requested(void) { return mg_quit_requested; }
static void mg_set_samples(int count) { g_requested_samples = count > 0 ? count : 0; }

static void mg_prepare(int count, const int *global, int opCount, const int *ops) {
    if (count != g_layer_count) {
        for (int i=0; i<MG_MAX_LAYERS; ++i) {
            g_layer_msaa[i] = nil; g_layer_resolve[i] = nil; g_layer_stencil[i] = nil;
            g_blur_msaa[i] = nil; g_blur_resolve[i] = nil; g_blur_stencil[i] = nil;
        }
    }
    g_layer_count = count;
    memcpy(g_layer_global, global, sizeof(int)*count);
    g_composite_count = opCount;
    memcpy(g_composite, ops, sizeof(int)*4*opCount);
}

static NSString *const kShaderSrc = @
    "#include <metal_stdlib>\n"
    "using namespace metal;\n"
    "struct VIn  { float2 pos [[attribute(0)]]; float4 col [[attribute(1)]]; };\n"
    "struct VOut { float4 pos [[position]]; float4 col; };\n"
    "vertex VOut v_main(VIn in [[stage_in]], constant float2 &vp [[buffer(1)]]) {\n"
    "  VOut o;\n"
    "  float2 ndc = float2( (in.pos.x / vp.x) * 2.0 - 1.0,\n"
    "                       1.0 - (in.pos.y / vp.y) * 2.0 );\n"
    "  o.pos = float4(ndc, 0.0, 1.0);\n"
    "  o.col = float4(in.col.rgb * in.col.a, in.col.a);\n"
    "  return o;\n"
    "}\n"
    "fragment float4 f_main(VOut in [[stage_in]]) { return in.col; }\n"
    "// Premultiplied Porter-Duff difference blend via framebuffer fetch.\n"
    "fragment float4 f_diff(VOut in [[stage_in]], float4 dst [[color(0)]]) {\n"
    "  float3 srcRGB = in.col.a > 0.0 ? in.col.rgb / in.col.a : float3(0.0);\n"
    "  float3 dstRGB = dst.a > 0.0 ? dst.rgb / dst.a : float3(0.0);\n"
    "  float3 rgb = (1.0-in.col.a)*dst.rgb + (1.0-dst.a)*in.col.rgb\n"
    "             + in.col.a*dst.a*abs(dstRGB-srcRGB);\n"
    "  return float4(rgb, in.col.a + dst.a * (1.0-in.col.a));\n"
    "}\n"
    "\n"
    "struct TIn  { float2 pos [[attribute(0)]]; float2 uv [[attribute(1)]]; float4 col [[attribute(2)]]; };\n"
    "struct TOut { float4 pos [[position]]; float2 uv; float4 col; };\n"
    "vertex TOut v_tex(TIn in [[stage_in]], constant float2 &vp [[buffer(1)]]) {\n"
    "  TOut o;\n"
    "  float2 ndc = float2( (in.pos.x / vp.x) * 2.0 - 1.0,\n"
    "                       1.0 - (in.pos.y / vp.y) * 2.0 );\n"
    "  o.pos = float4(ndc, 0.0, 1.0);\n"
    "  o.uv  = in.uv;\n"
    "  o.col = in.col;\n"
    "  return o;\n"
    "}\n"
    "fragment float4 f_tex(TOut in [[stage_in]], texture2d<float> atlas [[texture(0)]], sampler samp [[sampler(0)]]) {\n"
    "  float a = atlas.sample(samp, in.uv).r * in.col.a;\n"
    "  return float4(in.col.rgb * a, a);\n"
    "}\n"
    "\n"
    "static inline float msdf_median(float r, float g, float b) {\n"
    "  return max(min(r, g), min(max(r, g), b));\n"
    "}\n"
    "struct RIn  { float2 pos [[attribute(0)]]; float2 local [[attribute(1)]]; float2 halfSize [[attribute(2)]]; float radius [[attribute(3)]]; float softness [[attribute(4)]]; float thickness [[attribute(5)]]; float4 col [[attribute(6)]]; };\n"
    "struct ROut { float4 pos [[position]]; float2 local; float2 halfSize; float radius; float softness; float thickness; float4 col; };\n"
    "vertex ROut v_rrect(RIn in [[stage_in]], constant float2 &vp [[buffer(1)]]) {\n"
    "  ROut o;\n"
    "  float2 ndc = float2( (in.pos.x / vp.x) * 2.0 - 1.0,\n"
    "                       1.0 - (in.pos.y / vp.y) * 2.0 );\n"
    "  o.pos       = float4(ndc, 0.0, 1.0);\n"
    "  o.local     = in.local;\n"
    "  o.halfSize  = in.halfSize;\n"
    "  o.radius    = in.radius;\n"
    "  o.softness  = in.softness;\n"
    "  o.thickness = in.thickness;\n"
    "  o.col       = in.col;\n"
    "  return o;\n"
    "}\n"
    "static inline float sd_round_box(float2 p, float2 b, float r) {\n"
    "  float2 q = abs(p) - b + r;\n"
    "  return min(max(q.x, q.y), 0.0) + length(max(q, float2(0.0))) - r;\n"
    "}\n"
    "fragment float4 f_rrect(ROut in [[stage_in]]) {\n"
    "  float d = sd_round_box(in.local, in.halfSize, in.radius);\n"
    "  if (in.thickness > 0.0) d = abs(d) - in.thickness;\n"
    "  float aa = max(fwidth(d), 0.5);\n"
    "  float w  = aa + in.softness;\n"
    "  float coverage = 1.0 - smoothstep(-w, w, d);\n"
    "  float a = coverage * in.col.a;\n"
    "  return float4(in.col.rgb * a, a);\n"
    "}\n"
    "\n"
    "// Generic fullscreen vertex and caller-programmed layer compositor.\n"
    "struct CVOut { float4 pos [[position]]; float2 uv; };\n"
    "vertex CVOut v_compose(uint vid [[vertex_id]]) {\n"
    "  float2 p = float2((vid==1) ? 3.0 : -1.0, (vid==2) ? -3.0 : 1.0);\n"
    "  CVOut o; o.pos = float4(p, 0.0, 1.0);\n"
    "  o.uv = float2((p.x+1.0)*0.5, 1.0 - (p.y+1.0)*0.5);\n"
    "  return o;\n"
    "}\n"
    "fragment float4 f_compose(CVOut in [[stage_in]],\n"
    "                          array<texture2d<float>, 31> layers [[texture(0)]],\n"
    "                          constant int4 *ops [[buffer(0)]],\n"
    "                          constant int &count [[buffer(1)]],\n"
    "                          sampler samp [[sampler(0)]]) {\n"
    "  float4 dst = float4(0.0);\n"
    "  for (int i=0; i<count; ++i) {\n"
    "    int4 op = ops[i];\n"
    "    float4 src = layers[op.x].sample(samp, in.uv);\n"
    "    if (op.y >= 0) {\n"
    "      float coverage = layers[op.y].sample(samp, in.uv).a;\n"
    "      src *= op.z != 0 ? 1.0-coverage : coverage;\n"
    "    }\n"
    "    if (op.w == 1) {\n"
    "      dst = float4(mix(dst.rgb, float3(1.0)-dst.rgb, src.a), src.a+dst.a*(1.0-src.a));\n"
    "    } else {\n"
    "      dst = src + dst*(1.0-src.a);\n"
    "    }\n"
    "  }\n"
    "  return dst;\n"
    "}\n"
    "\n"
    "struct BIn  { float2 pos [[attribute(0)]]; float2 local [[attribute(1)]]; };\n"
    "struct BOut { float4 pos [[position]]; float2 local; };\n"
    "vertex BOut v_bgdots(BIn in [[stage_in]], constant float2 &vp [[buffer(1)]]) {\n"
    "  BOut o;\n"
    "  float2 ndc = float2((in.pos.x / vp.x) * 2.0 - 1.0,\n"
    "                      1.0 - (in.pos.y / vp.y) * 2.0);\n"
    "  o.pos   = float4(ndc, 0.0, 1.0);\n"
    "  o.local = in.local;\n"
    "  return o;\n"
    "}\n"
    "struct BgDotsUni { float4 params; float4 bg; float4 dot; };\n"
    "fragment float4 f_bgdots(BOut in [[stage_in]], constant BgDotsUni &u [[buffer(0)]]) {\n"
    "  float tile = u.params.x;\n"
    "  float dotR = u.params.y;\n"
    "  float2 anchored = in.local - u.params.zw;\n"
    "  float2 cell = anchored - float2(tile) * floor(anchored / float2(tile)) - float2(tile*0.5);\n"
    "  float d  = length(cell);\n"
    "  float mask = 1.0 - smoothstep(dotR - 0.5, dotR + 0.5, d);\n"
    "  float4 c = mix(u.bg, u.dot, mask);\n"
    "  return float4(c.rgb * c.a, c.a);\n"
    "}\n"
    "struct LatticeUni { float4 pitch; float4 origin; float4 bg; float4 ink; };\n"
    "static inline float lattice_coverage(float2 c, float halfWidth) {\n"
    "  float2 d = abs(fract(c+float2(0.5))-float2(0.5));\n"
    "  float2 pixels = d / max(fwidth(c), float2(0.000001));\n"
    "  float2 coverage = clamp(float2(halfWidth+0.5)-pixels,0.0,1.0);\n"
    "  return max(coverage.x,coverage.y);\n"
    "}\n"
    "fragment float4 f_lattice(BOut in [[stage_in]], constant LatticeUni &u [[buffer(0)]]) {\n"
    "  float2 uv = in.local-u.origin.xy;\n"
    "  float minor = lattice_coverage(uv/u.pitch.x,u.pitch.w);\n"
    "  float major = lattice_coverage(uv/(u.pitch.x*u.pitch.y),u.pitch.z);\n"
    "  float4 c = mix(u.bg,u.ink,max(minor,major));\n"
    "  return float4(c.rgb*c.a,c.a);\n"
    "}\n"
    "\n"
    "// Directional Gaussian filtering over a captured group.\n"
    "// per axis over the captured group composite. p.xy is the texel-sized\n"
    "// step direction, p.z the sigma in pixels; taps are normalised in-shader\n"
    "// so truncating the kernel at 2.5 sigma keeps unit energy.\n"
    "fragment float4 f_gauss(CVOut in [[stage_in]],\n"
    "                        texture2d<float> src [[texture(0)]],\n"
    "                        sampler samp [[sampler(0)]],\n"
    "                        constant float4 &p [[buffer(0)]]) {\n"
    "  float sigma = max(p.z, 0.001);\n"
    "  int R = int(clamp(ceil(sigma * 2.5), 1.0, 48.0));\n"
    "  float4 acc = float4(0.0);\n"
    "  float wsum = 0.0;\n"
    "  for (int i = -R; i <= R; i++) {\n"
    "    float w = exp(-0.5 * float(i) * float(i) / (sigma * sigma));\n"
    "    acc += src.sample(samp, in.uv + p.xy * float(i)) * w;\n"
    "    wsum += w;\n"
    "  }\n"
    "  return acc / wsum;\n"
    "}\n"
    "// Premultiplied source-over of a fullscreen texture — lands the blurred\n"
    "// group back on its original layer.\n"
    "fragment float4 f_blit(CVOut in [[stage_in]],\n"
    "                       texture2d<float> src [[texture(0)]],\n"
    "                       sampler samp [[sampler(0)]]) {\n"
    "  return src.sample(samp, in.uv);\n"
    "}\n"
    "\n"
    "fragment float4 f_msdf(TOut in [[stage_in]],\n"
    "                       texture2d<float> atlas [[texture(0)]],\n"
    "                       sampler samp [[sampler(0)]],\n"
    "                       constant float &screenPxRange [[buffer(0)]]) {\n"
    "  float3 msd = atlas.sample(samp, in.uv).rgb;\n"
    "  float sd = msdf_median(msd.r, msd.g, msd.b);\n"
    "  float w = max(screenPxRange, 1.0);\n"
    "  float opacity = clamp((sd - 0.5) * w + 0.5, 0.0, 1.0);\n"
    "  float a = opacity * in.col.a;\n"
    "  return float4(in.col.rgb * a, a);\n"
    "}\n";

static void mg_build_pipeline(void) {
    NSError *err = nil;
    id<MTLLibrary> lib = [g_device newLibraryWithSource:kShaderSrc options:nil error:&err];
    if (lib == nil) { NSLog(@"shader compile failed: %@", err); return; }

    MTLRenderPipelineDescriptor *pd = [[MTLRenderPipelineDescriptor alloc] init];
    pd.vertexFunction   = [lib newFunctionWithName:@"v_main"];
    pd.fragmentFunction = [lib newFunctionWithName:@"f_main"];
    pd.rasterSampleCount                               = g_sample_count;
    pd.stencilAttachmentPixelFormat                    = g_stencil_fmt;
    pd.colorAttachments[0].pixelFormat                 = MTLPixelFormatBGRA8Unorm;
    pd.colorAttachments[0].blendingEnabled             = YES;
    pd.colorAttachments[0].rgbBlendOperation           = MTLBlendOperationAdd;
    pd.colorAttachments[0].alphaBlendOperation         = MTLBlendOperationAdd;
    pd.colorAttachments[0].sourceRGBBlendFactor        = MTLBlendFactorOne;
    pd.colorAttachments[0].sourceAlphaBlendFactor      = MTLBlendFactorOne;
    pd.colorAttachments[0].destinationRGBBlendFactor   = MTLBlendFactorOneMinusSourceAlpha;
    pd.colorAttachments[0].destinationAlphaBlendFactor = MTLBlendFactorOneMinusSourceAlpha;

    MTLVertexDescriptor *vd = [[MTLVertexDescriptor alloc] init];
    vd.attributes[0].format = MTLVertexFormatFloat2; vd.attributes[0].offset = 0;  vd.attributes[0].bufferIndex = 0;
    vd.attributes[1].format = MTLVertexFormatFloat4; vd.attributes[1].offset = 8;  vd.attributes[1].bufferIndex = 0;
    vd.layouts[0].stride = 24;
    pd.vertexDescriptor = vd;

    g_pso = [g_device newRenderPipelineStateWithDescriptor:pd error:&err];
    if (g_pso == nil) { NSLog(@"pso failed: %@", err); }

    // Difference pipeline: same vertex layout as g_pso, but blending is
    // disabled — the shader composites via framebuffer fetch instead, so the
    // hardware blender must not double-apply.
    MTLRenderPipelineDescriptor *pdd = [[MTLRenderPipelineDescriptor alloc] init];
    pdd.vertexFunction   = [lib newFunctionWithName:@"v_main"];
    pdd.fragmentFunction = [lib newFunctionWithName:@"f_diff"];
    pdd.rasterSampleCount = g_sample_count;
    pdd.stencilAttachmentPixelFormat        = g_stencil_fmt;
    pdd.colorAttachments[0].pixelFormat     = MTLPixelFormatBGRA8Unorm;
    pdd.colorAttachments[0].blendingEnabled = NO;
    pdd.vertexDescriptor = vd;
    g_pso_diff = [g_device newRenderPipelineStateWithDescriptor:pdd error:&err];
    if (g_pso_diff == nil) { NSLog(@"diff pso failed: %@", err); }

    MTLRenderPipelineDescriptor *pdt = [[MTLRenderPipelineDescriptor alloc] init];
    pdt.vertexFunction   = [lib newFunctionWithName:@"v_tex"];
    pdt.fragmentFunction = [lib newFunctionWithName:@"f_tex"];
    pdt.rasterSampleCount = g_sample_count;
    pdt.stencilAttachmentPixelFormat                    = g_stencil_fmt;
    pdt.colorAttachments[0].pixelFormat                 = MTLPixelFormatBGRA8Unorm;
    pdt.colorAttachments[0].blendingEnabled             = YES;
    pdt.colorAttachments[0].rgbBlendOperation           = MTLBlendOperationAdd;
    pdt.colorAttachments[0].alphaBlendOperation         = MTLBlendOperationAdd;
    pdt.colorAttachments[0].sourceRGBBlendFactor        = MTLBlendFactorOne;
    pdt.colorAttachments[0].sourceAlphaBlendFactor      = MTLBlendFactorOne;
    pdt.colorAttachments[0].destinationRGBBlendFactor   = MTLBlendFactorOneMinusSourceAlpha;
    pdt.colorAttachments[0].destinationAlphaBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
    MTLVertexDescriptor *vdt = [[MTLVertexDescriptor alloc] init];
    vdt.attributes[0].format = MTLVertexFormatFloat2; vdt.attributes[0].offset = 0;  vdt.attributes[0].bufferIndex = 0;
    vdt.attributes[1].format = MTLVertexFormatFloat2; vdt.attributes[1].offset = 8;  vdt.attributes[1].bufferIndex = 0;
    vdt.attributes[2].format = MTLVertexFormatFloat4; vdt.attributes[2].offset = 16; vdt.attributes[2].bufferIndex = 0;
    vdt.layouts[0].stride = 32;
    pdt.vertexDescriptor = vdt;
    g_pso_tex = [g_device newRenderPipelineStateWithDescriptor:pdt error:&err];
    if (g_pso_tex == nil) { NSLog(@"tex pso failed: %@", err); }

    MTLRenderPipelineDescriptor *pdm = [pdt copy];
    pdm.fragmentFunction = [lib newFunctionWithName:@"f_msdf"];
    g_pso_msdf = [g_device newRenderPipelineStateWithDescriptor:pdm error:&err];
    if (g_pso_msdf == nil) { NSLog(@"msdf pso failed: %@", err); }

    MTLRenderPipelineDescriptor *pdr = [[MTLRenderPipelineDescriptor alloc] init];
    pdr.vertexFunction   = [lib newFunctionWithName:@"v_rrect"];
    pdr.fragmentFunction = [lib newFunctionWithName:@"f_rrect"];
    pdr.rasterSampleCount = g_sample_count;
    pdr.stencilAttachmentPixelFormat                    = g_stencil_fmt;
    pdr.colorAttachments[0].pixelFormat                 = MTLPixelFormatBGRA8Unorm;
    pdr.colorAttachments[0].blendingEnabled             = YES;
    pdr.colorAttachments[0].rgbBlendOperation           = MTLBlendOperationAdd;
    pdr.colorAttachments[0].alphaBlendOperation         = MTLBlendOperationAdd;
    pdr.colorAttachments[0].sourceRGBBlendFactor        = MTLBlendFactorOne;
    pdr.colorAttachments[0].sourceAlphaBlendFactor      = MTLBlendFactorOne;
    pdr.colorAttachments[0].destinationRGBBlendFactor   = MTLBlendFactorOneMinusSourceAlpha;
    pdr.colorAttachments[0].destinationAlphaBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
    MTLVertexDescriptor *vdr = [[MTLVertexDescriptor alloc] init];
    vdr.attributes[0].format = MTLVertexFormatFloat2; vdr.attributes[0].offset = 0;  vdr.attributes[0].bufferIndex = 0;
    vdr.attributes[1].format = MTLVertexFormatFloat2; vdr.attributes[1].offset = 8;  vdr.attributes[1].bufferIndex = 0;
    vdr.attributes[2].format = MTLVertexFormatFloat2; vdr.attributes[2].offset = 16; vdr.attributes[2].bufferIndex = 0;
    vdr.attributes[3].format = MTLVertexFormatFloat;  vdr.attributes[3].offset = 24; vdr.attributes[3].bufferIndex = 0;
    vdr.attributes[4].format = MTLVertexFormatFloat;  vdr.attributes[4].offset = 28; vdr.attributes[4].bufferIndex = 0;
    vdr.attributes[5].format = MTLVertexFormatFloat;  vdr.attributes[5].offset = 32; vdr.attributes[5].bufferIndex = 0;
    vdr.attributes[6].format = MTLVertexFormatFloat4; vdr.attributes[6].offset = 36; vdr.attributes[6].bufferIndex = 0;
    vdr.layouts[0].stride = 52;
    pdr.vertexDescriptor = vdr;
    g_pso_rrect = [g_device newRenderPipelineStateWithDescriptor:pdr error:&err];
    if (g_pso_rrect == nil) { NSLog(@"rrect pso failed: %@", err); }

    MTLRenderPipelineDescriptor *pdb = [[MTLRenderPipelineDescriptor alloc] init];
    pdb.vertexFunction   = [lib newFunctionWithName:@"v_bgdots"];
    pdb.fragmentFunction = [lib newFunctionWithName:@"f_bgdots"];
    pdb.rasterSampleCount = g_sample_count;
    pdb.stencilAttachmentPixelFormat                    = g_stencil_fmt;
    pdb.colorAttachments[0].pixelFormat                 = MTLPixelFormatBGRA8Unorm;
    pdb.colorAttachments[0].blendingEnabled             = YES;
    pdb.colorAttachments[0].rgbBlendOperation           = MTLBlendOperationAdd;
    pdb.colorAttachments[0].alphaBlendOperation         = MTLBlendOperationAdd;
    pdb.colorAttachments[0].sourceRGBBlendFactor        = MTLBlendFactorOne;
    pdb.colorAttachments[0].sourceAlphaBlendFactor      = MTLBlendFactorOne;
    pdb.colorAttachments[0].destinationRGBBlendFactor   = MTLBlendFactorOneMinusSourceAlpha;
    pdb.colorAttachments[0].destinationAlphaBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
    MTLVertexDescriptor *vdb = [[MTLVertexDescriptor alloc] init];
    vdb.attributes[0].format = MTLVertexFormatFloat2; vdb.attributes[0].offset = 0; vdb.attributes[0].bufferIndex = 0;
    vdb.attributes[1].format = MTLVertexFormatFloat2; vdb.attributes[1].offset = 8; vdb.attributes[1].bufferIndex = 0;
    vdb.layouts[0].stride = 16;
    pdb.vertexDescriptor = vdb;
    g_pso_bgdots = [g_device newRenderPipelineStateWithDescriptor:pdb error:&err];
    if (g_pso_bgdots == nil) { NSLog(@"bgdots pso failed: %@", err); }
    MTLRenderPipelineDescriptor *pdl = [pdb copy];
    pdl.fragmentFunction = [lib newFunctionWithName:@"f_lattice"];
    g_pso_lattice = [g_device newRenderPipelineStateWithDescriptor:pdl error:&err];
    if (g_pso_lattice == nil) { NSLog(@"lattice pso failed: %@", err); }


    // Clip-write pipeline: same vertex layout as the triangle path (pos+col),
    // colour writes disabled — only the stencil result matters. Stencil op is
    // baked into the depth/stencil state (g_dss_write / g_dss_clear), not the
    // pipeline, so one PSO covers both push and pop.
    MTLRenderPipelineDescriptor *pdc = [[MTLRenderPipelineDescriptor alloc] init];
    pdc.vertexFunction   = [lib newFunctionWithName:@"v_main"];
    pdc.fragmentFunction = [lib newFunctionWithName:@"f_main"];
    pdc.rasterSampleCount = g_sample_count;
    pdc.stencilAttachmentPixelFormat        = g_stencil_fmt;
    pdc.colorAttachments[0].pixelFormat     = MTLPixelFormatBGRA8Unorm;
    pdc.colorAttachments[0].writeMask       = MTLColorWriteMaskNone;
    pdc.vertexDescriptor = vd;
    g_pso_clip = [g_device newRenderPipelineStateWithDescriptor:pdc error:&err];
    if (g_pso_clip == nil) { NSLog(@"clip pso failed: %@", err); }

    // One stencil bit per nested clip. A non-zero clip sets its depth bit;
    // an even-odd clip toggles it once per contour. Comparison masks include
    // every parent bit, so an inner clip intersects rather than replacing its
    // parent. Pop only clears/toggles the current bit.
    MTLDepthStencilDescriptor *dsdN = [[MTLDepthStencilDescriptor alloc] init];
    g_dss_none = [g_device newDepthStencilStateWithDescriptor:dsdN];

    MTLStencilDescriptor *fail = [[MTLStencilDescriptor alloc] init];
    fail.stencilCompareFunction = MTLCompareFunctionNever;
    fail.writeMask = 0x00;
    MTLDepthStencilDescriptor *failD = [[MTLDepthStencilDescriptor alloc] init];
    failD.frontFaceStencil = fail; failD.backFaceStencil = fail;
    g_dss_clip_fail = [g_device newDepthStencilStateWithDescriptor:failD];

    for (int d = 1; d <= MG_CLIP_DEPTH; d++) {
        uint32_t bit = 1u << (d - 1);
        uint32_t parentMask = bit - 1u;
        uint32_t activeMask = (bit << 1u) - 1u;

        MTLStencilDescriptor *set = [[MTLStencilDescriptor alloc] init];
        set.stencilCompareFunction = MTLCompareFunctionEqual;
        set.depthStencilPassOperation = MTLStencilOperationReplace;
        set.readMask = parentMask;
        set.writeMask = bit;
        MTLDepthStencilDescriptor *setD = [[MTLDepthStencilDescriptor alloc] init];
        setD.frontFaceStencil = set; setD.backFaceStencil = set;
        g_dss_clip_set[d-1] = [g_device newDepthStencilStateWithDescriptor:setD];

        MTLStencilDescriptor *toggle = [[MTLStencilDescriptor alloc] init];
        toggle.stencilCompareFunction = MTLCompareFunctionEqual;
        toggle.depthStencilPassOperation = MTLStencilOperationInvert;
        toggle.readMask = parentMask;
        toggle.writeMask = bit;
        MTLDepthStencilDescriptor *toggleD = [[MTLDepthStencilDescriptor alloc] init];
        toggleD.frontFaceStencil = toggle; toggleD.backFaceStencil = toggle;
        g_dss_clip_toggle[d-1] = [g_device newDepthStencilStateWithDescriptor:toggleD];

        MTLStencilDescriptor *clear = [[MTLStencilDescriptor alloc] init];
        clear.stencilCompareFunction = MTLCompareFunctionEqual;
        clear.depthStencilPassOperation = MTLStencilOperationZero;
        clear.readMask = activeMask;
        clear.writeMask = bit;
        MTLDepthStencilDescriptor *clearD = [[MTLDepthStencilDescriptor alloc] init];
        clearD.frontFaceStencil = clear; clearD.backFaceStencil = clear;
        g_dss_clip_clear[d-1] = [g_device newDepthStencilStateWithDescriptor:clearD];

        MTLStencilDescriptor *test = [[MTLStencilDescriptor alloc] init];
        test.stencilCompareFunction = MTLCompareFunctionEqual;
        test.stencilFailureOperation = MTLStencilOperationKeep;
        test.depthStencilPassOperation = MTLStencilOperationKeep;
        test.readMask = activeMask;
        test.writeMask = 0x00;
        MTLDepthStencilDescriptor *testD = [[MTLDepthStencilDescriptor alloc] init];
        testD.frontFaceStencil = test; testD.backFaceStencil = test;
        g_dss_clip_test[d-1] = [g_device newDepthStencilStateWithDescriptor:testD];
    }

    // Composition resolves caller-selected layer textures to the drawable.
    MTLRenderPipelineDescriptor *pdz = [[MTLRenderPipelineDescriptor alloc] init];
    pdz.vertexFunction   = [lib newFunctionWithName:@"v_compose"];
    pdz.fragmentFunction = [lib newFunctionWithName:@"f_compose"];
    pdz.rasterSampleCount = 1;
    pdz.colorAttachments[0].pixelFormat     = MTLPixelFormatBGRA8Unorm;
    pdz.colorAttachments[0].blendingEnabled = NO;
    g_pso_compose = [g_device newRenderPipelineStateWithDescriptor:pdz error:&err];
    if (g_pso_compose == nil) { NSLog(@"compose pso failed: %@", err); }

    // Gaussian pass: fullscreen triangle between single-sample scratch
    // textures; no blending (it rewrites every pixel).
    MTLRenderPipelineDescriptor *pdg = [[MTLRenderPipelineDescriptor alloc] init];
    pdg.vertexFunction   = [lib newFunctionWithName:@"v_compose"];
    pdg.fragmentFunction = [lib newFunctionWithName:@"f_gauss"];
    pdg.rasterSampleCount = 1;
    pdg.colorAttachments[0].pixelFormat     = MTLPixelFormatBGRA8Unorm;
    pdg.colorAttachments[0].blendingEnabled = NO;
    g_pso_gauss = [g_device newRenderPipelineStateWithDescriptor:pdg error:&err];
    if (g_pso_gauss == nil) { NSLog(@"gauss pso failed: %@", err); }

    // Blit pass: premultiplied source-over of the blurred scratch back onto
    // an MSAA layer target, so it must match the layer pass's sample count
    // and stencil format.
    MTLRenderPipelineDescriptor *pdbl = [[MTLRenderPipelineDescriptor alloc] init];
    pdbl.vertexFunction   = [lib newFunctionWithName:@"v_compose"];
    pdbl.fragmentFunction = [lib newFunctionWithName:@"f_blit"];
    pdbl.rasterSampleCount = g_sample_count;
    pdbl.stencilAttachmentPixelFormat                    = g_stencil_fmt;
    pdbl.colorAttachments[0].pixelFormat                 = MTLPixelFormatBGRA8Unorm;
    pdbl.colorAttachments[0].blendingEnabled             = YES;
    pdbl.colorAttachments[0].rgbBlendOperation           = MTLBlendOperationAdd;
    pdbl.colorAttachments[0].alphaBlendOperation         = MTLBlendOperationAdd;
    pdbl.colorAttachments[0].sourceRGBBlendFactor        = MTLBlendFactorOne;
    pdbl.colorAttachments[0].sourceAlphaBlendFactor      = MTLBlendFactorOne;
    pdbl.colorAttachments[0].destinationRGBBlendFactor   = MTLBlendFactorOneMinusSourceAlpha;
    pdbl.colorAttachments[0].destinationAlphaBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
    g_pso_blit = [g_device newRenderPipelineStateWithDescriptor:pdbl error:&err];
    if (g_pso_blit == nil) { NSLog(@"blit pso failed: %@", err); }

    MTLSamplerDescriptor *sd = [[MTLSamplerDescriptor alloc] init];
    sd.minFilter = MTLSamplerMinMagFilterLinear;
    sd.magFilter = MTLSamplerMinMagFilterLinear;
    sd.sAddressMode = MTLSamplerAddressModeClampToEdge;
    sd.tAddressMode = MTLSamplerAddressModeClampToEdge;
    g_sampler = [g_device newSamplerStateWithDescriptor:sd];
}

typedef struct { double red, green, blue, alpha; } MgColor;
typedef struct {
    const char *name;
    MgColor background, tint;
    int opaque, titlebarTransparent, titleHidden, fullSizeContentView, movableByBackground;
    int backdrop, glass;
    double cornerRadius;
    int glassStyle, material, blendingMode, state;
} MgWindowAppearance;

static NSColor *mg_color(MgColor color) {
    return [NSColor colorWithSRGBRed:color.red green:color.green blue:color.blue alpha:color.alpha];
}

static void mg_setup(int winW, int winH, MgWindowAppearance appearance) {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

    NSRect frame = NSMakeRect(120, 120, winW, winH);
    NSUInteger style = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable
                     | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable;
    g_window = [[NSWindow alloc] initWithContentRect:frame
                                            styleMask:style
                                              backing:NSBackingStoreBuffered
                                                defer:NO];
    [g_window setTitle:@""];
    [g_window setReleasedWhenClosed:NO];
    [g_window setOpaque:appearance.opaque];
    [g_window setBackgroundColor:mg_color(appearance.background)];
    [g_window setTitlebarAppearsTransparent:appearance.titlebarTransparent];
    [g_window setTitleVisibility:appearance.titleHidden ? NSWindowTitleHidden : NSWindowTitleVisible];
    if (appearance.fullSizeContentView)
        [g_window setStyleMask:[g_window styleMask] | NSWindowStyleMaskFullSizeContentView];
    [g_window setMovableByWindowBackground:appearance.movableByBackground];

    NSRect contentRect = [[g_window contentView] bounds];

    g_device = MTLCreateSystemDefaultDevice();
    g_sample_count = mg_configured_sample_count(g_device);
    g_queue  = [g_device newCommandQueue];

    CGFloat scale = [g_window backingScaleFactor];
    g_layer = [CAMetalLayer layer];
    g_layer.device = g_device;
    g_layer.pixelFormat = MTLPixelFormatBGRA8Unorm;
    g_layer.framebufferOnly = YES;
    g_layer.opaque = appearance.opaque;
    g_layer.contentsScale = scale;
    g_layer.drawableSize = CGSizeMake(contentRect.size.width * scale, contentRect.size.height * scale);

    g_view = [[NSView alloc] initWithFrame:contentRect];
    [g_view setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
    [g_view setWantsLayer:YES];
    [g_view setLayer:g_layer];

    [g_window setAppearance:[NSAppearance appearanceNamed:[NSString stringWithUTF8String:appearance.name]]];

    Class glassClass = appearance.glass ? NSClassFromString(@"NSGlassEffectView") : Nil;
    if (!appearance.backdrop) {
        [g_window setContentView:g_view];
    } else if (glassClass != nil) {
        NSView *glass = [[glassClass alloc] initWithFrame:contentRect];
        [glass setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
        if ([glass respondsToSelector:@selector(setCornerRadius:)]) {
            [(NSObject *)glass setValue:@(appearance.cornerRadius) forKey:@"cornerRadius"];
        }
        if ([glass respondsToSelector:@selector(setStyle:)]) {
            [(NSObject *)glass setValue:@(appearance.glassStyle) forKey:@"style"];
        }
        if ([glass respondsToSelector:@selector(setTintColor:)]) {
            [(NSObject *)glass setValue:mg_color(appearance.tint) forKey:@"tintColor"];
        }
        if ([glass respondsToSelector:@selector(setContentView:)]) {
            [(NSObject *)glass setValue:g_view forKey:@"contentView"];
        } else {
            [glass addSubview:g_view];
        }
        [g_window setContentView:glass];
    } else {
        NSVisualEffectView *backdrop = [[NSVisualEffectView alloc] initWithFrame:contentRect];
        [backdrop setMaterial:(NSVisualEffectMaterial)appearance.material];
        [backdrop setBlendingMode:(NSVisualEffectBlendingMode)appearance.blendingMode];
        [backdrop setState:(NSVisualEffectState)appearance.state];
        [backdrop setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
        [backdrop addSubview:g_view];
        [g_window setContentView:backdrop];
    }

    mg_build_pipeline();

    g_winDelegate = [[MgWinDelegate alloc] init];
    [g_window setDelegate:g_winDelegate];
    g_appDelegate = [[MgAppDelegate alloc] init];
    [NSApp setDelegate:g_appDelegate];

    g_noDragView = [[MgNoDragView alloc] initWithFrame:NSZeroRect];
    [[g_window contentView] addSubview:g_noDragView positioned:NSWindowAbove relativeTo:nil];

    g_keydown_monitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyDown
                                          handler:^NSEvent *(NSEvent *e) {
        mg_set_key([e keyCode], 1);
        return e;
    }];
    g_keyup_monitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyUp
                                          handler:^NSEvent *(NSEvent *e) {
        mg_set_key([e keyCode], 0);
        return e;
    }];
    NSEventMask mmask = NSEventMaskMouseMoved | NSEventMaskLeftMouseDragged
                      | NSEventMaskLeftMouseDown | NSEventMaskLeftMouseUp;
    g_mouse_monitor = [NSEvent addLocalMonitorForEventsMatchingMask:mmask
                                          handler:^NSEvent *(NSEvent *e) {
        NSPoint p = [g_view convertPoint:[e locationInWindow] fromView:nil];
        // AppKit Y axis is bottom-up; canvas Y is top-down.
        g_mouse_x = (float)p.x;
        g_mouse_y = (float)([g_view bounds].size.height - p.y);
        NSEventType t = [e type];
        if (t == NSEventTypeLeftMouseDown) g_mouse_down = 1;
        else if (t == NSEventTypeLeftMouseUp) g_mouse_down = 0;
        return e;
    }];

    [g_window makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];
}

static void mg_resize(int winW, int winH) {
    if (g_window == nil || winW <= 0 || winH <= 0) return;
    [g_window setContentSize:NSMakeSize(winW, winH)];
}

// Background colour the base layer clears to on its first activation per
// frame. Stashed by mg_frame_begin and consumed by mg_select_layer when it
// opens the encoder for layer 0.
static float g_clear_r = 0, g_clear_g = 0, g_clear_b = 0, g_clear_a = 1;

static void mg_ensure_layer_textures(NSUInteger pxW, NSUInteger pxH) {
    BOOL needRealloc = (g_layer_msaa[0] == nil
                        || g_layer_msaa[0].width != pxW
                        || g_layer_msaa[0].height != pxH);
    if (!needRealloc) return;
    for (int i = 0; i < MG_LAYER_COUNT; i++) {
        MTLTextureDescriptor *td = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm
                                                                                       width:pxW
                                                                                      height:pxH
                                                                                   mipmapped:NO];
        if (mg_multisampling_enabled()) {
            td.textureType = MTLTextureType2DMultisample;
            td.sampleCount = g_sample_count;
            td.usage       = MTLTextureUsageRenderTarget;
        } else {
            td.usage       = MTLTextureUsageRenderTarget | MTLTextureUsageShaderRead;
        }
        td.storageMode = MTLStorageModePrivate;
        g_layer_msaa[i] = [g_device newTextureWithDescriptor:td];

        if (mg_multisampling_enabled()) {
            MTLTextureDescriptor *tr = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm
                                                                                           width:pxW
                                                                                          height:pxH
                                                                                       mipmapped:NO];
            tr.usage       = MTLTextureUsageRenderTarget | MTLTextureUsageShaderRead;
            tr.storageMode = MTLStorageModePrivate;
            g_layer_resolve[i] = [g_device newTextureWithDescriptor:tr];
        } else {
            g_layer_resolve[i] = g_layer_msaa[i];
        }

        MTLTextureDescriptor *ts = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:g_stencil_fmt
                                                                                       width:pxW
                                                                                      height:pxH
                                                                                   mipmapped:NO];
        if (mg_multisampling_enabled()) {
            ts.textureType = MTLTextureType2DMultisample;
            ts.sampleCount = g_sample_count;
        }
        ts.usage       = MTLTextureUsageRenderTarget;
        ts.storageMode = MTLStorageModePrivate;
        g_layer_stencil[i] = [g_device newTextureWithDescriptor:ts];
    }
}

// Reopening a layer preserves color and clears stencil; the Go renderer
// reinstalls active clip entries on every layer transition.
static int mg_select_layer(int t) {
    if (t < 0 || t >= MG_LAYER_COUNT || g_frame_cb == nil) return 0;
    if (t == g_current_layer) return 0;
    if (g_frame_enc != nil) {
        [g_frame_enc endEncoding];
        g_frame_enc = nil;
    }
    // During a depth-of-field capture every layer routes to the parallel
    // blur target set; the capture composites against transparent (the real
    // backdrop is already on the main base) so base never takes the clear
    // colour here.
    BOOL capturing = g_blur_capture && !g_layer_global[t];
    id<MTLTexture> __strong *msaa    = capturing ? g_blur_msaa    : g_layer_msaa;
    id<MTLTexture> __strong *resolve = capturing ? g_blur_resolve : g_layer_resolve;
    id<MTLTexture> __strong *stencil = capturing ? g_blur_stencil : g_layer_stencil;
    BOOL           *visited = capturing ? g_blur_visited : g_layer_visited;

    MTLRenderPassDescriptor *desc = [MTLRenderPassDescriptor renderPassDescriptor];
    mg_configure_resolved_color(desc.colorAttachments[0],
                                msaa[t],
                                resolve[t],
                                MTLStoreActionStoreAndMultisampleResolve);
    if (!visited[t]) {
        desc.colorAttachments[0].loadAction = MTLLoadActionClear;
        if (t == 0 && !capturing) {
            desc.colorAttachments[0].clearColor =
                MTLClearColorMake(g_clear_r*g_clear_a, g_clear_g*g_clear_a,
                                  g_clear_b*g_clear_a, g_clear_a);
        } else {
            desc.colorAttachments[0].clearColor = MTLClearColorMake(0, 0, 0, 0);
        }
        visited[t] = YES;
    } else {
        desc.colorAttachments[0].loadAction = MTLLoadActionLoad;
    }
    desc.stencilAttachment.texture      = stencil[t];
    desc.stencilAttachment.loadAction   = MTLLoadActionClear;
    desc.stencilAttachment.storeAction  = MTLStoreActionDontCare;
    desc.stencilAttachment.clearStencil = 0;

    g_frame_enc = [g_frame_cb renderCommandEncoderWithDescriptor:desc];
    [g_frame_enc setRenderPipelineState:g_pso];
    [g_frame_enc setDepthStencilState:g_dss_none];
    g_current_layer = t;
    // Don't reset g_vbuf_off / g_tbuf_off between passes: all passes in
    // the same command buffer share the buffer, and Metal defers reads
    // until commit. Rewinding here would let a later pass overwrite the
    // bytes an earlier pass already referenced, producing garbage geometry.
    return 1;
}

static void mg_ensure_blur_textures(NSUInteger pxW, NSUInteger pxH) {
    BOOL needRealloc = (g_blur_msaa[0] == nil
                        || g_blur_msaa[0].width != pxW
                        || g_blur_msaa[0].height != pxH);
    if (!needRealloc) return;
    for (int i = 0; i < MG_LAYER_COUNT; i++) {
        MTLTextureDescriptor *td = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm
                                                                                       width:pxW
                                                                                      height:pxH
                                                                                   mipmapped:NO];
        if (mg_multisampling_enabled()) {
            td.textureType = MTLTextureType2DMultisample;
            td.sampleCount = g_sample_count;
            td.usage       = MTLTextureUsageRenderTarget;
        } else {
            td.usage       = MTLTextureUsageRenderTarget | MTLTextureUsageShaderRead;
        }
        td.storageMode = MTLStorageModePrivate;
        g_blur_msaa[i] = [g_device newTextureWithDescriptor:td];

        if (mg_multisampling_enabled()) {
            MTLTextureDescriptor *tr = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm
                                                                                           width:pxW
                                                                                          height:pxH
                                                                                       mipmapped:NO];
            tr.usage       = MTLTextureUsageRenderTarget | MTLTextureUsageShaderRead;
            tr.storageMode = MTLStorageModePrivate;
            g_blur_resolve[i] = [g_device newTextureWithDescriptor:tr];
        } else {
            g_blur_resolve[i] = g_blur_msaa[i];
        }

        MTLTextureDescriptor *ts = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:g_stencil_fmt
                                                                                       width:pxW
                                                                                      height:pxH
                                                                                   mipmapped:NO];
        if (mg_multisampling_enabled()) {
            ts.textureType = MTLTextureType2DMultisample;
            ts.sampleCount = g_sample_count;
        }
        ts.usage       = MTLTextureUsageRenderTarget;
        ts.storageMode = MTLStorageModePrivate;
        g_blur_stencil[i] = [g_device newTextureWithDescriptor:ts];
    }
    MTLTextureDescriptor *tsc = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm
                                                                                   width:pxW
                                                                                  height:pxH
                                                                               mipmapped:NO];
    tsc.usage       = MTLTextureUsageRenderTarget | MTLTextureUsageShaderRead;
    tsc.storageMode = MTLStorageModePrivate;
    g_blur_scratchA = [g_device newTextureWithDescriptor:tsc];
    g_blur_scratchB = [g_device newTextureWithDescriptor:tsc];
}

// mg_blur_begin redirects all subsequent draws into the capture target set.
static void mg_blur_begin(void) {
    if (g_frame_cb == nil || g_blur_capture) return;
    CGSize ds = g_layer.drawableSize;
    if (ds.width <= 0 || ds.height <= 0) return;
    mg_ensure_blur_textures((NSUInteger)ds.width, (NSUInteger)ds.height);
    for (int i = 0; i < MG_LAYER_COUNT; i++) g_blur_visited[i] = NO;
    g_blur_prev_layer = (g_current_layer < 0) ? 0 : g_current_layer;
    g_blur_capture = YES;
    g_current_layer = -1;
    mg_select_layer(g_blur_prev_layer);
}

// Fullscreen pass helper: draw `pso` into `dst` sampling `src`, with the
// 16-byte fragment uniform `p` (ignored by f_blit).
static void mg_fullscreen_pass(id<MTLRenderPipelineState> pso,
                               id<MTLTexture> dst, id<MTLTexture> src,
                               const float *p) {
    MTLRenderPassDescriptor *d = [MTLRenderPassDescriptor renderPassDescriptor];
    d.colorAttachments[0].texture     = dst;
    d.colorAttachments[0].loadAction  = MTLLoadActionClear;
    d.colorAttachments[0].storeAction = MTLStoreActionStore;
    d.colorAttachments[0].clearColor  = MTLClearColorMake(0, 0, 0, 0);
    id<MTLRenderCommandEncoder> e = [g_frame_cb renderCommandEncoderWithDescriptor:d];
    [e setRenderPipelineState:pso];
    [e setFragmentTexture:src atIndex:0];
    [e setFragmentSamplerState:g_sampler atIndex:0];
    if (p != NULL) [e setFragmentBytes:p length:16 atIndex:0];
    [e drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:3];
    [e endEncoding];
}

// mg_blur_end composes the captured layers, gaussian-blurs the composite by
// `sigma_pts` (view points; converted to pixels here), source-overs it onto
// the layer that was active at mg_blur_begin, and reopens that layer for
// the remaining draws.
static void mg_blur_end(float sigma_pts) {
    if (g_frame_cb == nil || !g_blur_capture) return;
    if (g_frame_enc != nil) {
        [g_frame_enc endEncoding];
        g_frame_enc = nil;
    }
    // Untouched capture layers still need defined contents for the compose.
    for (int i = 0; i < MG_LAYER_COUNT; i++) {
        if (g_blur_visited[i]) continue;
        MTLRenderPassDescriptor *cd = [MTLRenderPassDescriptor renderPassDescriptor];
        mg_configure_resolved_color(cd.colorAttachments[0],
                                    g_blur_msaa[i],
                                    g_blur_resolve[i],
                                    MTLStoreActionMultisampleResolve);
        cd.colorAttachments[0].loadAction     = MTLLoadActionClear;
        cd.colorAttachments[0].clearColor     = MTLClearColorMake(0, 0, 0, 0);
        id<MTLRenderCommandEncoder> e = [g_frame_cb renderCommandEncoderWithDescriptor:cd];
        [e endEncoding];
        g_blur_visited[i] = YES;
    }

    // Compose the captured group with the same caller-supplied recipe.
    MTLRenderPassDescriptor *desc = [MTLRenderPassDescriptor renderPassDescriptor];
    desc.colorAttachments[0].texture     = g_blur_scratchA;
    desc.colorAttachments[0].loadAction  = MTLLoadActionClear;
    desc.colorAttachments[0].storeAction = MTLStoreActionStore;
    desc.colorAttachments[0].clearColor  = MTLClearColorMake(0, 0, 0, 0);
    id<MTLRenderCommandEncoder> ce = [g_frame_cb renderCommandEncoderWithDescriptor:desc];
    [ce setRenderPipelineState:g_pso_compose];
    for (int i = 0; i < MG_MAX_LAYERS; i++) {
        [ce setFragmentTexture:g_blur_resolve[i < MG_LAYER_COUNT ? i : 0] atIndex:i];
    }
    [ce setFragmentSamplerState:g_sampler atIndex:0];
    [ce setFragmentBytes:g_composite length:sizeof(int)*4*g_composite_count atIndex:0];
    [ce setFragmentBytes:&g_composite_count length:sizeof(int) atIndex:1];
    [ce drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:3];
    [ce endEncoding];

    CGFloat scale = g_layer.contentsScale > 0 ? g_layer.contentsScale : 1.0;
    float sigma_px = sigma_pts * (float)scale;
    float w = (float)g_blur_scratchA.width;
    float h = (float)g_blur_scratchA.height;
    id<MTLTexture> result = g_blur_scratchA;
    if (sigma_px >= 0.25f && w > 0 && h > 0) {
        float ph[4] = { 1.0f / w, 0.0f, sigma_px, 0.0f };
        mg_fullscreen_pass(g_pso_gauss, g_blur_scratchB, g_blur_scratchA, ph);
        float pv[4] = { 0.0f, 1.0f / h, sigma_px, 0.0f };
        mg_fullscreen_pass(g_pso_gauss, g_blur_scratchA, g_blur_scratchB, pv);
    }

    // Source-over the blurred composite onto the original layer target; the
    // pass loads the existing content (clearing only if this is somehow the
    // layer's first touch) and resolves when the active GPU path uses MSAA.
    g_blur_capture = NO;
    int t = g_blur_prev_layer;
    MTLRenderPassDescriptor *bd = [MTLRenderPassDescriptor renderPassDescriptor];
    mg_configure_resolved_color(bd.colorAttachments[0],
                                g_layer_msaa[t],
                                g_layer_resolve[t],
                                MTLStoreActionStoreAndMultisampleResolve);
    if (!g_layer_visited[t]) {
        bd.colorAttachments[0].loadAction = MTLLoadActionClear;
        bd.colorAttachments[0].clearColor = (t == 0)
            ? MTLClearColorMake(g_clear_r*g_clear_a, g_clear_g*g_clear_a,
                                g_clear_b*g_clear_a, g_clear_a)
            : MTLClearColorMake(0, 0, 0, 0);
        g_layer_visited[t] = YES;
    } else {
        bd.colorAttachments[0].loadAction = MTLLoadActionLoad;
    }
    bd.stencilAttachment.texture      = g_layer_stencil[t];
    bd.stencilAttachment.loadAction   = MTLLoadActionClear;
    bd.stencilAttachment.storeAction  = MTLStoreActionDontCare;
    bd.stencilAttachment.clearStencil = 0;
    id<MTLRenderCommandEncoder> be = [g_frame_cb renderCommandEncoderWithDescriptor:bd];
    [be setRenderPipelineState:g_pso_blit];
    [be setFragmentTexture:result atIndex:0];
    [be setFragmentSamplerState:g_sampler atIndex:0];
    [be drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:3];
    [be endEncoding];

    // Reopen the layer for whatever the frame draws next.
    g_current_layer = -1;
    mg_select_layer(t);
}

static int mg_frame_begin(float r, float g, float b, float a) {
    if (g_layer == nil) return 0;
    // Track the current view bounds — without this the metal layer keeps
    // the drawable size set at mg_setup, so the rendered content stretches
    // (or letterboxes wrong) once the user resizes the window. We resize
    // the drawable here; the per-frame SetViewport op already letterboxes
    // the scene into whatever drawable size it finds.
    if (g_window != nil && g_view != nil) {
        CGFloat scale = [g_window backingScaleFactor];
        if (scale <= 0) scale = 1.0;
        // Use the window's contentView bounds — that view always tracks the
        // window size. g_view may live inside an NSGlassEffectView's
        // contentView slot (set via KVC), which doesn't auto-resize its child
        // the way NSWindow does, so [g_view bounds] would stay frozen at the
        // initial size.
        NSSize wbs = [[g_window contentView] bounds].size;
        if (!NSEqualRects([g_view frame], NSMakeRect(0, 0, wbs.width, wbs.height))) {
            [g_view setFrame:NSMakeRect(0, 0, wbs.width, wbs.height)];
        }
        CGSize want = CGSizeMake(wbs.width * scale, wbs.height * scale);
        if (want.width > 0 && want.height > 0 &&
            (want.width != g_layer.drawableSize.width ||
             want.height != g_layer.drawableSize.height)) {
            g_layer.drawableSize  = want;
            g_layer.contentsScale = scale;
        }
    }
    CGSize ds = g_layer.drawableSize;
    if (ds.width <= 0 || ds.height <= 0) return 0;
    CGFloat scale = g_layer.contentsScale > 0 ? g_layer.contentsScale : 1.0;
    g_vp_w = (float)(ds.width / scale);
    g_vp_h = (float)(ds.height / scale);

    g_frame_drawable = [g_layer nextDrawable];
    if (g_frame_drawable == nil) return 0;

    mg_ensure_layer_textures((NSUInteger)ds.width, (NSUInteger)ds.height);
    for (int i = 0; i < MG_LAYER_COUNT; i++) g_layer_visited[i] = NO;
    g_current_layer = -1;
    g_clear_r = r; g_clear_g = g; g_clear_b = b; g_clear_a = a;

    g_frame_cb = [g_queue commandBuffer];
    g_vbuf_off = 0;
    g_tbuf_off = 0;
    // Pre-warm base so callers that draw before any pushLayer have a target.
    mg_select_layer(0);
    return 1;
}

// Grow the buffer to at least `need` bytes total capacity, copying any
// already-written data (offset 0..used) into the new allocation so commands
// already encoded against the old buffer keep referencing valid data — Metal
// retains the old buffer until the command buffer completes, but if we're
// using the SAME global pointer for future draws within the same encode pass
// we must hand them the new buffer; the old draws still reference the old one.
static id<MTLBuffer> mg_ensure_buf(id<MTLBuffer> buf, NSUInteger *cap, NSUInteger used, NSUInteger need) {
    if (buf != nil && *cap >= need) return buf;
    NSUInteger newCap = (*cap < 4096) ? 4096 : *cap;
    while (newCap < need) newCap *= 2;
    id<MTLBuffer> nb = [g_device newBufferWithLength:newCap options:MTLResourceStorageModeShared];
    if (buf != nil && used > 0) {
        memcpy([nb contents], [buf contents], used);
    }
    *cap = newCap;
    return nb;
}

static void mg_atlas_upload(int x, int y, int w, int h, const unsigned char *bytes) {
    if (w <= 0 || h <= 0) return;
    if (g_atlas == nil || g_atlas.width != (NSUInteger)w || g_atlas.height != (NSUInteger)h) {
        MTLTextureDescriptor *td = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatRGBA8Unorm
                                                                                   width:w height:h mipmapped:NO];
        td.usage = MTLTextureUsageShaderRead;
        td.storageMode = MTLStorageModeShared;
        g_atlas = [g_device newTextureWithDescriptor:td];
    }
    MTLRegion region = MTLRegionMake2D((NSUInteger)x, (NSUInteger)y, (NSUInteger)w, (NSUInteger)h);
    [g_atlas replaceRegion:region mipmapLevel:0 withBytes:bytes bytesPerRow:(NSUInteger)(w * 4)];
}

// Align to 16 bytes (safe for any vertex stride we use).
static inline NSUInteger mg_align16(NSUInteger n) { return (n + 15u) & ~(NSUInteger)15u; }

static void mg_draw_rrect(const float *verts, int count) {
    if (g_frame_enc == nil || count <= 0) return;
    NSUInteger bytes = (NSUInteger)count * 52;
    NSUInteger off   = g_tbuf_off;
    NSUInteger need  = off + bytes;
    g_tbuf = mg_ensure_buf(g_tbuf, &g_tbuf_cap, off, need);
    memcpy((char *)[g_tbuf contents] + off, verts, bytes);
    g_tbuf_off = mg_align16(need);
    float vp[2] = { g_vp_w, g_vp_h };
    [g_frame_enc setRenderPipelineState:g_pso_rrect];
    [g_frame_enc setVertexBuffer:g_tbuf offset:off atIndex:0];
    [g_frame_enc setVertexBytes:vp length:sizeof(vp) atIndex:1];
    [g_frame_enc drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:count];
    [g_frame_enc setRenderPipelineState:g_pso];
}

// uni layout: [tile, dotR, 0, 0,  bg.rgba,  dot.rgba]  (12 floats, 48 bytes)
static void mg_draw_pattern(const float *verts, int count, const float *uni, int lattice) {
    if (g_frame_enc == nil || count <= 0) return;
    NSUInteger bytes = (NSUInteger)count * 16;
    NSUInteger off   = g_tbuf_off;
    NSUInteger need  = off + bytes;
    g_tbuf = mg_ensure_buf(g_tbuf, &g_tbuf_cap, off, need);
    memcpy((char *)[g_tbuf contents] + off, verts, bytes);
    g_tbuf_off = mg_align16(need);
    float vp[2] = { g_vp_w, g_vp_h };
    [g_frame_enc setRenderPipelineState:lattice ? g_pso_lattice : g_pso_bgdots];
    [g_frame_enc setVertexBuffer:g_tbuf offset:off atIndex:0];
    [g_frame_enc setVertexBytes:vp length:sizeof(vp) atIndex:1];
    [g_frame_enc setFragmentBytes:uni length:lattice ? 64 : 48 atIndex:0];
    [g_frame_enc drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:count];
    [g_frame_enc setRenderPipelineState:g_pso];
}

static void mg_draw_msdf(const float *verts, int count, float screenPxRange) {
    if (g_frame_enc == nil || count <= 0) return;
    NSUInteger bytes = (NSUInteger)count * 32;
    NSUInteger off   = g_tbuf_off;
    NSUInteger need  = off + bytes;
    g_tbuf = mg_ensure_buf(g_tbuf, &g_tbuf_cap, off, need);
    memcpy((char *)[g_tbuf contents] + off, verts, bytes);
    g_tbuf_off = mg_align16(need);
    float vp[2] = { g_vp_w, g_vp_h };
    [g_frame_enc setRenderPipelineState:g_pso_msdf];
    [g_frame_enc setVertexBuffer:g_tbuf offset:off atIndex:0];
    [g_frame_enc setVertexBytes:vp length:sizeof(vp) atIndex:1];
    [g_frame_enc setFragmentTexture:g_atlas atIndex:0];
    [g_frame_enc setFragmentSamplerState:g_sampler atIndex:0];
    [g_frame_enc setFragmentBytes:&screenPxRange length:sizeof(float) atIndex:0];
    [g_frame_enc drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:count];
    [g_frame_enc setRenderPipelineState:g_pso];
}

static void mg_draw_triangles(const float *verts, int count) {
    if (g_frame_enc == nil || count <= 0) return;
    NSUInteger bytes = (NSUInteger)count * 24;
    NSUInteger off   = g_vbuf_off;
    NSUInteger need  = off + bytes;
    g_vbuf = mg_ensure_buf(g_vbuf, &g_vbuf_cap, off, need);
    memcpy((char *)[g_vbuf contents] + off, verts, bytes);
    g_vbuf_off = mg_align16(need);
    float vp[2] = { g_vp_w, g_vp_h };
    [g_frame_enc setVertexBuffer:g_vbuf offset:off atIndex:0];
    [g_frame_enc setVertexBytes:vp length:sizeof(vp) atIndex:1];
    [g_frame_enc drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:count];
}

static void mg_draw_triangles_diff(const float *verts, int count) {
    if (g_frame_enc == nil || count <= 0) return;
    NSUInteger bytes = (NSUInteger)count * 24;
    NSUInteger off   = g_vbuf_off;
    NSUInteger need  = off + bytes;
    g_vbuf = mg_ensure_buf(g_vbuf, &g_vbuf_cap, off, need);
    memcpy((char *)[g_vbuf contents] + off, verts, bytes);
    g_vbuf_off = mg_align16(need);
    float vp[2] = { g_vp_w, g_vp_h };
    [g_frame_enc setRenderPipelineState:g_pso_diff];
    [g_frame_enc setVertexBuffer:g_vbuf offset:off atIndex:0];
    [g_frame_enc setVertexBytes:vp length:sizeof(vp) atIndex:1];
    [g_frame_enc drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:count];
    [g_frame_enc setRenderPipelineState:g_pso];
}

// Clip modes: set a non-zero contour, toggle one even-odd contour, or clear a
// non-zero contour while popping. The depth selects the dedicated stencil bit.
static void mg_draw_clip(const float *verts, int count, int depth, int mode) {
    if (g_frame_enc == nil || count <= 0 || depth < 1 || depth > MG_CLIP_DEPTH) return;
    NSUInteger bytes = (NSUInteger)count * 24;
    NSUInteger off   = g_vbuf_off;
    NSUInteger need  = off + bytes;
    g_vbuf = mg_ensure_buf(g_vbuf, &g_vbuf_cap, off, need);
    memcpy((char *)[g_vbuf contents] + off, verts, bytes);
    g_vbuf_off = mg_align16(need);
    float vp[2] = { g_vp_w, g_vp_h };
    uint32_t bit = 1u << (depth - 1);
    uint32_t activeMask = (bit << 1u) - 1u;
    uint32_t parentMask = bit - 1u;
    id<MTLDepthStencilState> state = g_dss_clip_set[depth-1];
    uint32_t reference = activeMask;
    if (mode == 1) {
        state = g_dss_clip_toggle[depth-1];
        reference = parentMask;
    } else if (mode == 2) {
        state = g_dss_clip_clear[depth-1];
    }
    [g_frame_enc setRenderPipelineState:g_pso_clip];
    [g_frame_enc setDepthStencilState:state];
    [g_frame_enc setStencilReferenceValue:reference];
    [g_frame_enc setVertexBuffer:g_vbuf offset:off atIndex:0];
    [g_frame_enc setVertexBytes:vp length:sizeof(vp) atIndex:1];
    [g_frame_enc drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:count];
    [g_frame_enc setRenderPipelineState:g_pso];
}

static void mg_set_clip_depth(int depth) {
    if (g_frame_enc == nil) return;
    if (depth <= 0) {
        [g_frame_enc setDepthStencilState:g_dss_none];
        return;
    }
    if (depth > MG_CLIP_DEPTH) {
        [g_frame_enc setDepthStencilState:g_dss_clip_fail];
        [g_frame_enc setStencilReferenceValue:0];
        return;
    }
    uint32_t activeMask = (1u << depth) - 1u;
    [g_frame_enc setDepthStencilState:g_dss_clip_test[depth-1]];
    [g_frame_enc setStencilReferenceValue:activeMask];
}

static int mg_clip_depth_limit(void) { return MG_CLIP_DEPTH; }

static void mg_frame_end(void) {
    if (g_frame_cb == nil) return;
    if (g_frame_enc != nil) {
        [g_frame_enc endEncoding];
        g_frame_enc = nil;
    }
    // Clear unvisited targets before sampling them in the composition pass.
    for (int i = 0; i < MG_LAYER_COUNT; i++) {
        if (g_layer_visited[i]) continue;
        MTLRenderPassDescriptor *cd = [MTLRenderPassDescriptor renderPassDescriptor];
        mg_configure_resolved_color(cd.colorAttachments[0],
                                    g_layer_msaa[i],
                                    g_layer_resolve[i],
                                    MTLStoreActionMultisampleResolve);
        cd.colorAttachments[0].loadAction     = MTLLoadActionClear;
        cd.colorAttachments[0].clearColor     = (i == 0)
            ? MTLClearColorMake(g_clear_r*g_clear_a, g_clear_g*g_clear_a,
                                g_clear_b*g_clear_a, g_clear_a)
            : MTLClearColorMake(0, 0, 0, 0);
        id<MTLRenderCommandEncoder> e = [g_frame_cb renderCommandEncoderWithDescriptor:cd];
        [e endEncoding];
        g_layer_visited[i] = YES;
    }
    MTLRenderPassDescriptor *desc = [MTLRenderPassDescriptor renderPassDescriptor];
    desc.colorAttachments[0].texture     = g_frame_drawable.texture;
    desc.colorAttachments[0].loadAction  = MTLLoadActionClear;
    desc.colorAttachments[0].storeAction = MTLStoreActionStore;
    desc.colorAttachments[0].clearColor  = MTLClearColorMake(0, 0, 0, 0);
    id<MTLRenderCommandEncoder> ce = [g_frame_cb renderCommandEncoderWithDescriptor:desc];
    [ce setRenderPipelineState:g_pso_compose];
    for (int i = 0; i < MG_MAX_LAYERS; i++) {
        [ce setFragmentTexture:g_layer_resolve[i < MG_LAYER_COUNT ? i : 0] atIndex:i];
    }
    [ce setFragmentSamplerState:g_sampler atIndex:0];
    [ce setFragmentBytes:g_composite length:sizeof(int)*4*g_composite_count atIndex:0];
    [ce setFragmentBytes:&g_composite_count length:sizeof(int) atIndex:1];
    [ce drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:3];
    [ce endEncoding];

    [g_frame_cb presentDrawable:g_frame_drawable];
    [g_frame_cb commit];
    // Reused shared vertex buffers cannot be overwritten while the GPU reads.
    [g_frame_cb waitUntilCompleted];
    g_frame_cb = nil; g_frame_drawable = nil; g_current_layer = -1;
}

static void mg_start_tick(int fps) {
    mg_stop_tick();
    mg_quit_requested = 0;
    if (fps <= 0) fps = 60;
    g_tickTarget = [[MgTickTarget alloc] init];
    g_tickTimer = [NSTimer timerWithTimeInterval:(1.0/(double)fps)
                                          target:g_tickTarget
                                        selector:@selector(tick:)
                                        userInfo:nil
                                         repeats:YES];
    [[NSRunLoop mainRunLoop] addTimer:g_tickTimer forMode:NSRunLoopCommonModes];
}

static void mg_run(void) {
    [NSApp run];
    mg_stop_tick();
    g_tickTarget = nil;
    if (g_keydown_monitor != nil) [NSEvent removeMonitor:g_keydown_monitor];
    if (g_keyup_monitor != nil) [NSEvent removeMonitor:g_keyup_monitor];
    if (g_mouse_monitor != nil) [NSEvent removeMonitor:g_mouse_monitor];
    g_keydown_monitor = nil; g_keyup_monitor = nil; g_mouse_monitor = nil;
    memset(g_key_down, 0, sizeof(g_key_down));
    g_mouse_down = 0;
    [g_window orderOut:nil];
    [g_window setDelegate:nil];
    [NSApp setDelegate:nil];
    g_window = nil; g_view = nil; g_noDragView = nil; g_layer = nil;
    g_winDelegate = nil; g_appDelegate = nil;
    for (int i=0; i<MG_MAX_LAYERS; ++i) {
        g_layer_msaa[i] = nil; g_layer_resolve[i] = nil; g_layer_stencil[i] = nil;
        g_blur_msaa[i] = nil; g_blur_resolve[i] = nil; g_blur_stencil[i] = nil;
    }
    g_blur_scratchA = nil; g_blur_scratchB = nil; g_blur_capture = NO;
    g_vbuf = nil; g_tbuf = nil; g_vbuf_cap = 0; g_tbuf_cap = 0;
    g_atlas = nil; g_frame_cb = nil; g_frame_drawable = nil; g_frame_enc = nil;
    g_pso = nil; g_pso_diff = nil; g_pso_tex = nil; g_pso_msdf = nil;
    g_pso_rrect = nil; g_pso_bgdots = nil; g_pso_lattice = nil;
    g_pso_compose = nil; g_pso_gauss = nil; g_pso_blit = nil; g_pso_clip = nil;
    g_dss_none = nil; g_dss_clip_fail = nil;
    for (int i=0; i<MG_CLIP_DEPTH; ++i) {
        g_dss_clip_set[i] = nil; g_dss_clip_toggle[i] = nil;
        g_dss_clip_clear[i] = nil; g_dss_clip_test[i] = nil;
    }
    g_sampler = nil; g_queue = nil; g_device = nil;
}

static float mg_backing_scale(void) {
    if (g_window == nil) return 1.0f;
    return (float)[g_window backingScaleFactor];
}
static float mg_vp_width(void)  { return g_vp_w; }
static float mg_vp_height(void) { return g_vp_h; }
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/i-am-the-slime/purescript-native-graphics/drawing"
)

// Prepare selects the caller's target layout before FrameBegin allocates textures.
// Metal's texture argument limit is 31; uniform bytes limit the recipe to 256 steps.
func Prepare(frame *drawing.Drawing) error {
	count := len(frame.Layers)
	if count < 1 || count > 31 {
		return fmt.Errorf("Metal requires 1..31 layers, got %d", count)
	}
	if len(frame.Composite) < 1 || len(frame.Composite) > 256 {
		return fmt.Errorf("Metal requires 1..256 composition steps, got %d", len(frame.Composite))
	}
	var globals [31]C.int
	var ops [256 * 4]C.int
	for i, layer := range frame.Layers {
		if layer.Global {
			globals[i] = 1
		}
	}
	for i, op := range frame.Composite {
		if op.Source < 0 || op.Source >= count || op.Mask < -1 || op.Mask >= count || op.Blend < 0 || op.Blend > 1 {
			return fmt.Errorf("invalid Metal composition step %d", i)
		}
		ops[i*4], ops[i*4+1], ops[i*4+3] = C.int(op.Source), C.int(op.Mask), C.int(op.Blend)
		if op.InvertMask {
			ops[i*4+2] = 1
		}
	}
	C.mg_prepare(C.int(count), &globals[0], C.int(len(frame.Composite)), &ops[0])
	return nil
}

var tickFn func()

//export goRenderTick
func goRenderTick() {
	if tickFn != nil {
		tickFn()
	}
}

// SetTickFunc registers the per-frame callback invoked by the AppKit timer.
// Call before StartTick.
func SetTickFunc(f func()) { tickFn = f }

func nativeFlag(value bool) C.int {
	if value {
		return 1
	}
	return 0
}

func nativeColor(value Color) C.MgColor {
	return C.MgColor{red: C.double(value.Red), green: C.double(value.Green), blue: C.double(value.Blue), alpha: C.double(value.Alpha)}
}

func Setup(winW, winH int) {
	C.mg_set_samples(C.int(configuration.SampleCount))
	a := configuration.Appearance
	name := C.CString(a.Name)
	defer C.free(unsafe.Pointer(name))
	C.mg_setup(C.int(winW), C.int(winH), C.MgWindowAppearance{
		name: name, background: nativeColor(a.Background), tint: nativeColor(a.Tint),
		opaque: nativeFlag(a.Opaque), titlebarTransparent: nativeFlag(a.TitlebarTransparent),
		titleHidden: nativeFlag(a.TitleHidden), fullSizeContentView: nativeFlag(a.FullSizeContentView),
		movableByBackground: nativeFlag(a.MovableByBackground), backdrop: nativeFlag(a.Backdrop),
		glass: nativeFlag(a.Glass), cornerRadius: C.double(a.CornerRadius),
		glassStyle: C.int(a.GlassStyle), material: C.int(a.Material), blendingMode: C.int(a.BlendingMode), state: C.int(a.State),
	})
	title := C.CString(configuration.QuitMenuTitle)
	key := C.CString(configuration.QuitKeyEquivalent)
	C.mg_install_menu(title, key)
	C.free(unsafe.Pointer(title))
	C.free(unsafe.Pointer(key))
	if len(configuration.IconPNG) > 0 {
		C.mg_set_icon((*C.uchar)(unsafe.Pointer(&configuration.IconPNG[0])), C.int(len(configuration.IconPNG)))
	}
	uploadTextAtlas()
}

func SetTitle(title string) {
	value := C.CString(title)
	defer C.free(unsafe.Pointer(value))
	C.mg_set_title(value)
}

func SetDragExclusion(x, y, w, h float64) {
	C.mg_set_drag_exclusion(C.double(x), C.double(y), C.double(w), C.double(h))
}

func Stop()                 { C.mg_request_stop() }
func QuitRequested() bool   { return C.mg_get_quit_requested() != 0 }
func Resize(winW, winH int) { C.mg_resize(C.int(winW), C.int(winH)) }
func StartTick(fps int)     { C.mg_start_tick(C.int(fps)) }
func Run() {
	C.mg_run()
	tickFn = nil
}
func BackingScale() float32 { return float32(C.mg_backing_scale()) }

// MouseState returns view-point coordinates with top-down Y and left-button state.
func MouseState() (x, y float32, down bool) {
	return float32(C.mg_get_mouse_x()), float32(C.mg_get_mouse_y()), C.mg_get_mouse_down() != 0
}

// KeyDown reports whether a macOS virtual keycode is currently pressed.
func KeyDown(code int) bool { return C.mg_get_key(C.int(code)) != 0 }

// Native virtual keycodes.
const (
	KeySpace  = 49
	KeyLeft   = 123
	KeyRight  = 124
	KeyHome   = 115
	KeyEnd    = 119
	KeyT      = 17
	KeyQ      = 12
	KeyEscape = 53
)

// ViewportSize returns the current logical (point, not pixel) drawable size.
// Updated each frame by mg_frame_begin from the window's contentView bounds.
func ViewportSize() (float32, float32) {
	return float32(C.mg_vp_width()), float32(C.mg_vp_height())
}

func FrameBegin(r, g, b, a float32) bool {
	return C.mg_frame_begin(C.float(r), C.float(g), C.float(b), C.float(a)) != 0
}
func FrameEnd() { C.mg_frame_end() }

func drawTriangles(verts []float32) {
	if len(verts) == 0 {
		return
	}
	C.mg_draw_triangles((*C.float)(unsafe.Pointer(&verts[0])), C.int(len(verts)/6))
}

func drawTrianglesDiff(verts []float32) {
	if len(verts) == 0 {
		return
	}
	C.mg_draw_triangles_diff((*C.float)(unsafe.Pointer(&verts[0])), C.int(len(verts)/6))
}

func atlasUpload(x, y, w, h int, bytes []byte) {
	if len(bytes) == 0 {
		return
	}
	C.mg_atlas_upload(C.int(x), C.int(y), C.int(w), C.int(h),
		(*C.uchar)(unsafe.Pointer(&bytes[0])))
}

func selectLayer(t int) bool { return C.mg_select_layer(C.int(t)) != 0 }

func blurBegin()            { C.mg_blur_begin() }
func blurEnd(sigma float32) { C.mg_blur_end(C.float(sigma)) }

const (
	clipModeSet = iota
	clipModeToggle
	clipModeClear
)

func drawClipPath(verts []float32, depth, mode int) {
	if len(verts) == 0 {
		return
	}
	C.mg_draw_clip(
		(*C.float)(unsafe.Pointer(&verts[0])),
		C.int(len(verts)/6),
		C.int(depth),
		C.int(mode),
	)
}

func setClipDepth(depth int) {
	C.mg_set_clip_depth(C.int(depth))
}

func clipDepthLimit() int {
	return int(C.mg_clip_depth_limit())
}

func drawRRect(verts []float32) {
	if len(verts) == 0 {
		return
	}
	C.mg_draw_rrect((*C.float)(unsafe.Pointer(&verts[0])), C.int(len(verts)/13))
}

func drawBgDots(verts, uni []float32) {
	if len(verts) == 0 {
		return
	}
	C.mg_draw_pattern((*C.float)(unsafe.Pointer(&verts[0])), C.int(len(verts)/4), (*C.float)(unsafe.Pointer(&uni[0])), 0)
}

func drawLattice(verts, uni []float32) {
	if len(verts) == 0 {
		return
	}
	C.mg_draw_pattern((*C.float)(unsafe.Pointer(&verts[0])), C.int(len(verts)/4), (*C.float)(unsafe.Pointer(&uni[0])), 1)
}

func drawMSDF(verts []float32, screenPxRange float32) {
	if len(verts) == 0 {
		return
	}
	C.mg_draw_msdf((*C.float)(unsafe.Pointer(&verts[0])), C.int(len(verts)/8), C.float(screenPxRange))
}
