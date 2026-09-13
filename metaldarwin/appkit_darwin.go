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
static id<MTLTexture> g_atlas = nil;
static id<MTLRenderPipelineState> g_pso_compose = nil;
static id<MTLSamplerState> g_sampler = nil;
static id<MTLRenderPipelineState> g_pso_gauss = nil;
static id<MTLRenderPipelineState> g_pso_blit = nil;
typedef struct {
    void *color;
    void *resolve;
    void *stencil;
} MgTarget;
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

// Resource operations deliberately do not cache targets, choose passes or
// remember layer visits. The PureScript render closure owns those decisions.
static void *mg_retain(id value) { return (__bridge_retained void *)value; }
static void mg_release(void *value) {
    if (value != NULL) { id released = (__bridge_transfer id)value; (void)released; }
}

static MgTarget mg_target_create(int width, int height, int multisample) {
    MTLTextureDescriptor *td = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm
        width:width height:height mipmapped:NO];
    td.usage = MTLTextureUsageRenderTarget | MTLTextureUsageShaderRead;
    td.storageMode = MTLStorageModePrivate;
    id<MTLTexture> resolve = [g_device newTextureWithDescriptor:td];
    id<MTLTexture> color = resolve;
    id<MTLTexture> stencil = nil;
    if (multisample) {
        if (mg_multisampling_enabled()) {
            td.textureType = MTLTextureType2DMultisample;
            td.sampleCount = g_sample_count;
            td.usage = MTLTextureUsageRenderTarget;
            color = [g_device newTextureWithDescriptor:td];
        }
        td.pixelFormat = g_stencil_fmt;
        td.usage = MTLTextureUsageRenderTarget;
        stencil = [g_device newTextureWithDescriptor:td];
    }
    return (MgTarget){mg_retain(color), mg_retain(resolve), mg_retain(stencil)};
}
static void mg_target_release(MgTarget target) {
    mg_release(target.color); mg_release(target.resolve); mg_release(target.stencil);
}
static void mg_encoder_end(void) {
    if (g_frame_enc != nil) { [g_frame_enc endEncoding]; g_frame_enc = nil; }
}
static void mg_pass_begin(MgTarget target, int clear, const float *color) {
    mg_encoder_end();
    MTLRenderPassDescriptor *d = [MTLRenderPassDescriptor renderPassDescriptor];
    mg_configure_resolved_color(d.colorAttachments[0], (__bridge id)target.color,
        (__bridge id)target.resolve, MTLStoreActionStoreAndMultisampleResolve);
    d.colorAttachments[0].loadAction = clear ? MTLLoadActionClear : MTLLoadActionLoad;
    d.colorAttachments[0].clearColor = MTLClearColorMake(color[0], color[1], color[2], color[3]);
    d.stencilAttachment.texture = (__bridge id)target.stencil;
    d.stencilAttachment.loadAction = MTLLoadActionClear;
    d.stencilAttachment.storeAction = MTLStoreActionDontCare;
    g_frame_enc = [g_frame_cb renderCommandEncoderWithDescriptor:d];
    [g_frame_enc setDepthStencilState:g_dss_none];
}
static void mg_compose(const MgTarget *sources, int count, const int *ops, int opCount, MgTarget destination, int drawable) {
    mg_encoder_end();
    MTLRenderPassDescriptor *d = [MTLRenderPassDescriptor renderPassDescriptor];
    d.colorAttachments[0].texture = drawable ? g_frame_drawable.texture : (__bridge id)destination.resolve;
    d.colorAttachments[0].loadAction = MTLLoadActionClear;
    d.colorAttachments[0].storeAction = MTLStoreActionStore;
    id<MTLRenderCommandEncoder> e = [g_frame_cb renderCommandEncoderWithDescriptor:d];
    [e setRenderPipelineState:g_pso_compose];
    for (int i=0; i<31; i++) [e setFragmentTexture:(__bridge id)sources[i<count ? i : 0].resolve atIndex:i];
    [e setFragmentSamplerState:g_sampler atIndex:0];
    [e setFragmentBytes:ops length:sizeof(int)*4*opCount atIndex:0];
    [e setFragmentBytes:&opCount length:sizeof(int) atIndex:1];
    [e drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:3];
    [e endEncoding];
}
static void mg_filter(MgTarget source, MgTarget destination, const float *uniform) {
    mg_encoder_end();
    MTLRenderPassDescriptor *d = [MTLRenderPassDescriptor renderPassDescriptor];
    d.colorAttachments[0].texture = (__bridge id)destination.resolve;
    d.colorAttachments[0].loadAction = MTLLoadActionClear;
    d.colorAttachments[0].storeAction = MTLStoreActionStore;
    id<MTLRenderCommandEncoder> e = [g_frame_cb renderCommandEncoderWithDescriptor:d];
    [e setRenderPipelineState:g_pso_gauss];
    [e setFragmentTexture:(__bridge id)source.resolve atIndex:0];
    [e setFragmentSamplerState:g_sampler atIndex:0];
    [e setFragmentBytes:uniform length:16 atIndex:0];
    [e drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:3];
    [e endEncoding];
}
static void mg_blit(MgTarget source) {
    [g_frame_enc setRenderPipelineState:g_pso_blit];
    [g_frame_enc setFragmentTexture:(__bridge id)source.resolve atIndex:0];
    [g_frame_enc setFragmentSamplerState:g_sampler atIndex:0];
    [g_frame_enc drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:3];
}
static int mg_frame_begin(void) {
    if (g_layer == nil) return 0;
    CGFloat scale = [g_window backingScaleFactor];
    if (scale <= 0) scale = 1;
    NSSize bounds = [[g_window contentView] bounds].size;
    if (bounds.width <= 0 || bounds.height <= 0) return 0;
    [g_view setFrame:NSMakeRect(0, 0, bounds.width, bounds.height)];
    g_layer.drawableSize = CGSizeMake(bounds.width*scale, bounds.height*scale);
    g_layer.contentsScale = scale;
    g_vp_w = bounds.width; g_vp_h = bounds.height;
    g_frame_drawable = [g_layer nextDrawable];
    if (g_frame_drawable == nil) return 0;
    g_frame_cb = [g_queue commandBuffer];
    return 1;
}
static void mg_frame_end(void) {
    mg_encoder_end();
    [g_frame_cb presentDrawable:g_frame_drawable];
    [g_frame_cb commit];
    [g_frame_cb waitUntilCompleted];
    g_frame_cb = nil; g_frame_drawable = nil;
}
static void *mg_buffer_create(int capacity) {
    return mg_retain([g_device newBufferWithLength:capacity options:MTLResourceStorageModeShared]);
}
static void mg_draw(void *buffer, int offset, const float *vertices, int floats,
                    int pipeline, int stride, const float *uniform, int uniformCount) {
    id<MTLBuffer> b = (__bridge id)buffer;
    memcpy((char *)[b contents]+offset, vertices, sizeof(float)*floats);
    id<MTLRenderPipelineState> states[] = {g_pso, g_pso_diff, g_pso_rrect, g_pso_bgdots, g_pso_lattice, g_pso_msdf, g_pso_clip};
    [g_frame_enc setRenderPipelineState:states[pipeline]];
    [g_frame_enc setVertexBuffer:b offset:offset atIndex:0];
    float vp[] = {g_vp_w, g_vp_h};
    [g_frame_enc setVertexBytes:vp length:sizeof(vp) atIndex:1];
    if (uniformCount > 0) [g_frame_enc setFragmentBytes:uniform length:sizeof(float)*uniformCount atIndex:0];
    if (pipeline == 5) {
        [g_frame_enc setFragmentTexture:g_atlas atIndex:0];
        [g_frame_enc setFragmentSamplerState:g_sampler atIndex:0];
    }
    [g_frame_enc drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:floats/stride];
}
static void mg_stencil(int state, int depth, int reference) {
    id<MTLDepthStencilState> value = g_dss_none;
    if (state == 1) value = g_dss_clip_fail;
    if (state == 2) value = g_dss_clip_set[depth-1];
    if (state == 3) value = g_dss_clip_toggle[depth-1];
    if (state == 4) value = g_dss_clip_clear[depth-1];
    if (state == 5) value = g_dss_clip_test[depth-1];
    [g_frame_enc setDepthStencilState:value];
    [g_frame_enc setStencilReferenceValue:reference];
}
static void mg_atlas_upload(int x, int y, int w, int h, const unsigned char *bytes) {
    MTLTextureDescriptor *td = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatRGBA8Unorm
        width:w height:h mipmapped:NO];
    td.usage = MTLTextureUsageShaderRead;
    td.storageMode = MTLStorageModeShared;
    g_atlas = [g_device newTextureWithDescriptor:td];
    [g_atlas replaceRegion:MTLRegionMake2D(x,y,w,h) mipmapLevel:0 withBytes:bytes bytesPerRow:w*4];
}

static void mg_start_tick(int fps) {
    mg_stop_tick();
    mg_quit_requested = 0;
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
    // Render-closure-owned buffers and targets are released by PureScript.
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
static float mg_vp_width(void) { return (float)[[g_window contentView] bounds].size.width; }
static float mg_vp_height(void) { return (float)[[g_window contentView] bounds].size.height; }
*/
import "C"

import "unsafe"

var tickFn func()
var cleanupFn func()

func SetCleanupFunc(f func()) { cleanupFn = f }

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
	if cleanupFn != nil {
		cleanupFn()
		cleanupFn = nil
	}
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
// Query live window bounds so the PureScript frame callback receives a resize immediately.
func ViewportSize() (float32, float32) {
	return float32(C.mg_vp_width()), float32(C.mg_vp_height())
}

func FrameBegin() bool { return C.mg_frame_begin() != 0 }
func FrameEnd()        { C.mg_frame_end() }

type Target struct{ native C.MgTarget }
type Buffer struct{ native unsafe.Pointer }

func NewTarget(width, height int, multisample bool) *Target {
	target := &Target{C.mg_target_create(C.int(width), C.int(height), nativeFlag(multisample))}
	if target.native.color == nil || target.native.resolve == nil || (multisample && target.native.stencil == nil) {
		ReleaseTarget(target)
		panic("Metal target allocation failed")
	}
	return target
}
func ReleaseTarget(target *Target) {
	C.mg_target_release(target.native)
	target.native = C.MgTarget{}
}
func NewBuffer(capacity int) *Buffer {
	buffer := &Buffer{C.mg_buffer_create(C.int(capacity))}
	if buffer.native == nil {
		panic("Metal vertex buffer allocation failed")
	}
	return buffer
}
func ReleaseBuffer(buffer *Buffer) {
	C.mg_release(buffer.native)
	buffer.native = nil
}
func BeginPass(target *Target, clear bool, color []float32) {
	C.mg_pass_begin(target.native, nativeFlag(clear), (*C.float)(unsafe.Pointer(&color[0])))
}
func EndPass() { C.mg_encoder_end() }
func Compose(sources []*Target, ops []int32, destination *Target, drawable bool) {
	targets := make([]C.MgTarget, len(sources))
	for i, source := range sources {
		targets[i] = source.native
	}
	var dst C.MgTarget
	if destination != nil {
		dst = destination.native
	}
	C.mg_compose(&targets[0], C.int(len(targets)), (*C.int)(unsafe.Pointer(&ops[0])), C.int(len(ops)/4), dst, nativeFlag(drawable))
}
func Filter(source, destination *Target, uniform []float32) {
	C.mg_filter(source.native, destination.native, (*C.float)(unsafe.Pointer(&uniform[0])))
}
func Blit(source *Target) { C.mg_blit(source.native) }
func Stencil(state, depth, reference int) {
	C.mg_stencil(C.int(state), C.int(depth), C.int(reference))
}
func Draw(buffer *Buffer, offset int, vertices []float32, pipeline, stride int, uniform []float32) {
	if len(vertices) == 0 {
		return
	}
	var u *C.float
	if len(uniform) > 0 {
		u = (*C.float)(unsafe.Pointer(&uniform[0]))
	}
	C.mg_draw(buffer.native, C.int(offset), (*C.float)(unsafe.Pointer(&vertices[0])), C.int(len(vertices)),
		C.int(pipeline), C.int(stride), u, C.int(len(uniform)))
}

func atlasUpload(x, y, w, h int, bytes []byte) {
	if len(bytes) == 0 {
		return
	}
	C.mg_atlas_upload(C.int(x), C.int(y), C.int(w), C.int(h),
		(*C.uchar)(unsafe.Pointer(&bytes[0])))
}
