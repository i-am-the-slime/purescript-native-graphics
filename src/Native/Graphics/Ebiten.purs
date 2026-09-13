module Native.Graphics.Ebiten (create) where

import Prelude

import Data.Array as Array
import Data.Foldable (foldl, traverse_)
import Data.Int as Int
import Data.List (List(..))
import Data.Map (Map)
import Data.Map as Map
import Data.Maybe (Maybe(..), fromMaybe)
import Data.Number as Number
import Data.String.Common as String
import Data.String.Pattern (Pattern(..))
import Effect (Effect)
import Effect.Exception (throw)
import Effect.Ref (Ref)
import Effect.Ref as Ref
import Native.Graphics.Ebiten.Primitives as GPU
import Native.Graphics.Font as Font
import Native.Graphics.Geometry as Geometry
import Native.Graphics.Types (BlendMode(..), CirclePatternSpec, Color, Command(..), Drawing, FillRule(..), Image, LatticeSpec, LayerId(..), LineCap(..), LineJoin(..), Path, PathOp(..), Rect, StrokeStyle, TextAlign(..), TextBaseline(..), TextSpec, Transform)

type Surface = Ref (Map Int Image)
type Group = { surface :: Surface, alpha :: Number, blur :: Number, clip :: Maybe Image }
type CachedFace = { size :: Number, source :: GPU.FaceSource, face :: GPU.Face }
type State =
  { width :: Int
  , height :: Int
  , camera :: Transform
  , transforms :: List Transform
  , layers :: List Int
  , drawing :: Drawing
  , root :: Surface
  , groups :: List Group
  , surfaces :: List Surface
  , images :: List Image
  , zero :: Maybe Image
  , shaders :: Map String GPU.Shader
  , faces :: Map String CachedFace
  }

transparent :: Color
transparent = { red: 0.0, green: 0.0, blue: 0.0, alpha: 0.0 }

-- Each closure owns its GPU resources. No process-global renderer state.
create :: Effect (Image -> Drawing -> Boolean -> Effect Unit)
create = do
  root <- Ref.new Map.empty
  state <- Ref.new
    { width: 0
    , height: 0
    , camera: Geometry.identity
    , transforms: Nil
    , layers: Cons 0 Nil
    , drawing: { commands: [], layers: [], composite: [], clear: transparent }
    , root
    , groups: Nil
    , surfaces: Nil
    , images: Nil
    , zero: Nothing
    , shaders: Map.empty
    , faces: Map.empty
    }
  pure \target drawing overlay -> do
    prepare state target drawing
    traverse_ (execute state) drawing.commands
    closeGroups state
    resolved <- acquireImage state
    current <- Ref.read state
    resolve state resolved current.root
    unless overlay $ GPU.fillBackground { target, color: drawing.clear }
    GPU.drawImage { target, source: resolved, alpha: 1.0 }
    releaseImage state resolved

prepare :: Ref State -> Image -> Drawing -> Effect Unit
prepare state target drawing = do
  previous <- Ref.read state
  releaseSurface state previous.root
  size <- GPU.dimensions target
  when (size.width /= previous.width || size.height /= previous.height) do
    current <- Ref.read state
    traverse_ GPU.disposeImage current.images
    traverse_ GPU.disposeImage current.zero
    Ref.modify_ (_ { images = Nil, zero = Nothing }) state
  Ref.modify_
    ( _
        { width = size.width
        , height = size.height
        , drawing = drawing
        , camera = Geometry.identity
        , transforms = Nil
        , layers = Cons 0 Nil
        }
    )
    state

acquireImage :: Ref State -> Effect Image
acquireImage state = do
  current <- Ref.read state
  image <- case current.images of
    Cons image rest -> do
      Ref.modify_ (_ { images = rest }) state
      pure image
    Nil -> GPU.newImage { width: current.width, height: current.height }
  GPU.clearImage image
  pure image

releaseImage :: Ref State -> Image -> Effect Unit
releaseImage state image = Ref.modify_ (\s -> s { images = Cons image s.images }) state

releaseSurface :: Ref State -> Surface -> Effect Unit
releaseSurface state surface = do
  images <- Ref.read surface
  traverse_ (releaseImage state) images
  Ref.write Map.empty surface

zeroImage :: Ref State -> Effect Image
zeroImage state = do
  current <- Ref.read state
  case current.zero of
    Just image -> pure image
    Nothing -> do
      image <- GPU.newImage { width: current.width, height: current.height }
      Ref.modify_ (_ { zero = Just image }) state
      pure image

activeTarget :: Ref State -> Effect Image
activeTarget state = do
  current <- Ref.read state
  let
    index = case current.layers of
      Cons value _ -> value
      Nil -> 0
    global = fromMaybe false $ _.global <$> Array.index current.drawing.layers index
    surface = case current.groups of
      Cons group _ | not global -> group.surface
      _ -> current.root
  when (index < 0 || index >= max 1 (Array.length current.drawing.layers)) $
    throw "graphics: Ebiten layer index outside drawing configuration"
  images <- Ref.read surface
  case Map.lookup index images of
    Just image -> pure image
    Nothing -> do
      image <- acquireImage state
      Ref.modify_ (Map.insert index image) surface
      pure image

pushGroup :: Ref State -> Number -> Number -> Maybe Image -> Effect Unit
pushGroup state alpha blur clip = do
  current <- Ref.read state
  surface <- case current.surfaces of
    Cons surface rest -> do
      Ref.modify_ (_ { surfaces = rest }) state
      pure surface
    Nil -> Ref.new Map.empty
  Ref.modify_ (\s -> s { groups = Cons { surface, alpha, blur, clip } s.groups }) state

popGroup :: Ref State -> Effect Unit
popGroup state = do
  current <- Ref.read state
  case current.groups of
    Nil -> pure unit
    Cons group rest -> do
      Ref.modify_ (_ { groups = rest }) state
      resolved <- acquireImage state
      resolve state resolved group.surface
      when (group.blur > 0.0) do
        temporary <- acquireImage state
        blurInto state temporary resolved group.blur 1.0 0.0
        GPU.clearImage resolved
        blurInto state resolved temporary group.blur 0.0 1.0
        releaseImage state temporary
      target <- activeTarget state
      case group.clip of
        Nothing -> GPU.drawImage { target, source: resolved, alpha: group.alpha }
        Just clip -> do
          shader <- loadShader state "clip" GPU.clipSource
          fullShader state target shader [ resolved, clip ] [ scalar "Alpha" group.alpha ]
      releaseImage state resolved
      traverse_ (releaseImage state) group.clip
      releaseSurface state group.surface
      Ref.modify_ (\s -> s { surfaces = Cons group.surface s.surfaces }) state

closeGroups :: Ref State -> Effect Unit
closeGroups state = do
  current <- Ref.read state
  case current.groups of
    Nil -> pure unit
    Cons _ _ -> popGroup state *> closeGroups state

resolve :: Ref State -> Image -> Surface -> Effect Unit
resolve state target surface = do
  current <- Ref.read state
  images <- Ref.read surface
  scratch <- Ref.new Nothing
  traverse_ (step images scratch) current.drawing.composite
  Ref.read scratch >>= traverse_ (releaseImage state)
  where
  step images scratch recipe = case Map.lookup (layerIndex recipe.source) images of
    Nothing -> pure unit
    Just source -> case recipe.mask, recipe.blend of
      Nothing, SourceOver -> GPU.drawImage { target, source, alpha: 1.0 }
      _, _ -> do
        mask <- case recipe.mask >>= (\id -> Map.lookup (layerIndex id) images) of
          Just image -> pure image
          Nothing -> zeroImage state
        cached <- Ref.read scratch
        temporary <- case cached of
          Just image -> GPU.clearImage image *> pure image
          Nothing -> do
            image <- acquireImage state
            Ref.write (Just image) scratch
            pure image
        shader <- loadShader state "composite" GPU.compositeSource
        let
          masked = case recipe.mask of
            Nothing -> 0.0
            Just _ -> 1.0
          blend = case recipe.blend of
            SourceOver -> 0.0
            Invert -> 1.0
        fullShader state temporary shader [ target, source, mask ]
          [ scalar "Masked" masked, scalar "InvertMask" (flag recipe.invertMask), scalar "Blend" blend ]
        GPU.clearImage target
        GPU.drawImage { target, source: temporary, alpha: 1.0 }

layerIndex :: LayerId -> Int
layerIndex (LayerId index) = index

flag :: Boolean -> Number
flag value = if value then 1.0 else 0.0

scalar :: String -> Number -> GPU.Uniform
scalar name value = { name, values: [ value ] }

vector :: String -> Array Number -> GPU.Uniform
vector name values = { name, values }

deviceColor :: Color -> GPU.RGBA8
deviceColor color =
  let
    alpha = Int.floor (color.alpha * 255.0)
    channel value = (Int.floor (value * 255.0) * alpha) `div` 255
  in
    { red: channel color.red, green: channel color.green, blue: channel color.blue, alpha }

colorUniform :: String -> Color -> GPU.Uniform
colorUniform name color =
  let
    bytes = deviceColor color
    channel value = Int.toNumber value / 255.0
  in
    vector name [ channel bytes.red, channel bytes.green, channel bytes.blue, channel bytes.alpha ]

loadShader :: Ref State -> String -> String -> Effect GPU.Shader
loadShader state key source = do
  current <- Ref.read state
  case Map.lookup key current.shaders of
    Just shader -> pure shader
    Nothing -> do
      shader <- GPU.compileShader source
      Ref.modify_ (\s -> s { shaders = Map.insert key shader s.shaders }) state
      pure shader

fullShader :: Ref State -> Image -> GPU.Shader -> Array Image -> Array GPU.Uniform -> Effect Unit
fullShader state target shader images uniforms = do
  current <- Ref.read state
  GPU.drawShader { target, shader, images, uniforms, width: current.width, height: current.height, x: 0.0, y: 0.0 }

blurInto :: Ref State -> Image -> Image -> Number -> Number -> Number -> Effect Unit
blurInto state target source radius dx dy = do
  let extent = Int.ceil (radius * 3.0)
  current <- Ref.read state
  let key = "blur:" <> show extent
  shader <- case Map.lookup key current.shaders of
    Just cached -> pure cached
    Nothing -> loadShader state key (GPU.blurSource extent)
  fullShader state target shader [ source ] [ scalar "Sigma" radius, vector "Direction" [ dx, dy ] ]

buildPath :: Transform -> Path -> Effect GPU.NativePath
buildPath camera operations = do
  path <- GPU.newPath
  traverse_ (append path) operations
  pure path
  where
  point = Geometry.apply camera
  append path = case _ of
    MoveTo x y -> GPU.moveTo { path, point: point x y }
    LineTo x y -> GPU.lineTo { path, point: point x y }
    QuadTo cx cy x y -> GPU.quadTo { path, control: point cx cy, point: point x y }
    CubicTo ax ay bx by x y -> GPU.cubicTo { path, control1: point ax ay, control2: point bx by, point: point x y }
    ClosePath -> GPU.closePath path

stroke :: Image -> GPU.NativePath -> Transform -> Color -> StrokeStyle -> Effect Unit
stroke target path camera color style = when (style.width > 0.0) $
  GPU.strokePath { target, path, color: deviceColor color, width: style.width * Geometry.xScale camera, join: joinTag style.join, cap: capTag style.cap }
  where
  joinTag = case _ of
    RoundJoin -> 0
    BevelJoin -> 1
    MiterJoin -> 2
  capTag = case _ of
    ButtCap -> 0
    RoundCap -> 1
    SquareCap -> 2

execute :: Ref State -> Command -> Effect Unit
execute state command = do
  current <- Ref.read state
  case command of
    FillPath operations color -> do
      target <- activeTarget state
      path <- buildPath current.camera operations
      GPU.fillPath { target, path, color: deviceColor color, evenOdd: false }
    StrokePath operations color style -> when (style.width > 0.0) do
      target <- activeTarget state
      path <- buildPath current.camera operations
      stroke target path current.camera color style
    FillStrokePath operations fill outline style -> do
      target <- activeTarget state
      path <- buildPath current.camera operations
      GPU.fillPath { target, path, color: deviceColor fill, evenOdd: false }
      stroke target path current.camera outline style
    DrawText spec -> drawText state spec
    PushTransform transform -> Ref.modify_
      ( \s -> s
          { transforms = Cons s.camera s.transforms, camera = Geometry.compose s.camera transform }
      )
      state
    PopTransform -> case current.transforms of
      Nil -> pure unit
      Cons camera rest -> Ref.modify_ (_ { camera = camera, transforms = rest }) state
    PushClip operations rule -> do
      mask <- acquireImage state
      path <- buildPath current.camera operations
      let
        evenOdd = case rule of
          NonZero -> false
          EvenOdd -> true
      GPU.fillPath { target: mask, path, color: { red: 255, green: 255, blue: 255, alpha: 255 }, evenOdd }
      pushGroup state 1.0 0.0 (Just mask)
    PopClip -> popGroup state
    PushAlpha alpha -> pushGroup state alpha 0.0 Nothing
    PopAlpha -> popGroup state
    PushLayer id -> Ref.modify_ (\s -> s { layers = Cons (layerIndex id) s.layers }) state
    PopLayer -> case current.layers of
      Cons _ rest@(Cons _ _) -> Ref.modify_ (_ { layers = rest }) state
      _ -> pure unit
    Viewport rect -> Ref.modify_ (_ { camera = Geometry.viewport (Int.toNumber current.width) (Int.toNumber current.height) rect }) state
    Clear color -> do
      target <- activeTarget state
      GPU.fillImage { target, color: deviceColor color }
    CirclePattern spec -> circlePattern state spec
    PushBlur radius -> pushGroup state 1.0 (radius * Geometry.xScale current.camera) Nothing
    PopBlur -> popGroup state
    LineLattice spec -> lineLattice state spec

patternRect :: Ref State -> Rect -> GPU.Shader -> Array GPU.Uniform -> Effect Unit
patternRect state rect shader uniforms = do
  current <- Ref.read state
  let
    start = Geometry.apply current.camera rect.x rect.y
    end = Geometry.apply current.camera (rect.x + rect.width) (rect.y + rect.height)
    x = min start.x end.x
    y = min start.y end.y
    width = Int.ceil (Number.abs (end.x - start.x))
    height = Int.ceil (Number.abs (end.y - start.y))
  when (width > 0 && height > 0) do
    target <- activeTarget state
    GPU.drawShader { target, shader, images: [], uniforms, width, height, x, y }

circlePattern :: Ref State -> CirclePatternSpec -> Effect Unit
circlePattern state spec = do
  current <- Ref.read state
  let camera = current.camera
  when (spec.tile > 0.0 && camera.a /= 0.0 && camera.d /= 0.0) do
    shader <- loadShader state "circle" GPU.circleSource
    patternRect state spec.rect shader
      [ vector "Scale" [ camera.a, camera.d ]
      , vector "Translation" [ camera.tx, camera.ty ]
      , vector "Origin" [ spec.originX, spec.originY ]
      , scalar "Tile" spec.tile
      , scalar "Radius" spec.radius
      , colorUniform "BgColor" spec.background
      , colorUniform "InkColor" spec.ink
      ]

lineLattice :: Ref State -> LatticeSpec -> Effect Unit
lineLattice state spec = do
  current <- Ref.read state
  let
    width = Int.toNumber current.width
    height = Int.toNumber current.height
    pitch = spec.pitchFraction * min width height
    majorPitch = pitch * spec.majorEvery
  when (pitch > 0.0 && majorPitch > 0.0) do
    shader <- loadShader state "lattice" GPU.latticeSource
    patternRect state spec.rect shader
      [ scalar "TilePx" pitch
      , scalar "MajorEvery" spec.majorEvery
      , scalar "MajorHalf" spec.majorWidth
      , scalar "MinorHalf" spec.minorWidth
      , vector "Origin" [ remainder (spec.anchorX * width) majorPitch, remainder (spec.anchorY * height) majorPitch ]
      , colorUniform "BgColor" spec.background
      , colorUniform "InkColor" spec.ink
      ]
  where
  remainder x y = x - Number.trunc (x / y) * y

faceFor :: Ref State -> String -> Number -> Effect GPU.Face
faceFor state families size = do
  family <- Font.resolveFamily families
  current <- Ref.read state
  source <- GPU.faceSource family
  case Map.lookup family current.faces of
    Just cached | cached.size == size && GPU.sameFaceSource cached.source source -> pure cached.face
    _ -> do
      face <- GPU.newFace { source, size }
      Ref.modify_ (\s -> s { faces = Map.insert family { size, source, face } s.faces }) state
      pure face

drawText :: Ref State -> TextSpec -> Effect Unit
drawText state spec = when (spec.text /= "" && spec.size > 0.0) do
  current <- Ref.read state
  let size = spec.size * Geometry.xScale current.camera
  when (size > 0.0) do
    face <- faceFor state spec.font size
    target <- activeTarget state
    metrics <- GPU.faceMetrics face
    let
      position = Geometry.apply current.camera spec.x spec.y
      offset = size * spec.boldOffset
      -- Ebiten's zero line spacing overlays lines. Normal alignment is per
      -- line; synthetic bold historically aligns the complete measured block.
      lines =
        if spec.boldOffset /= 0.0 then [ spec.text ]
        else case spec.align of
          AlignLeft -> [ spec.text ]
          _ -> foldl (\parts separator -> Array.concatMap (String.split (Pattern separator)) parts)
            [ spec.text ]
            [ "\n", "\r", "\x000b", "\x000c", "\x0085", "\x2028", "\x2029" ]
    traverse_ (drawLine target face metrics position offset) lines
  where
  drawLine target face metrics position offset content = when (content /= "") do
    width <-
      if spec.boldOffset == 0.0 then pure 0.0
      else (_ + offset) <$> GPU.measureText { face, text: content }
    let
      -- Native alignment determines the subpixel glyph-rasterization origin;
      -- moving the final image instead changes antialiasing.
      align =
        if spec.boldOffset /= 0.0 then 0
        else case spec.align of
          AlignLeft -> 0
          AlignCenter -> 1
          AlignRight -> 2
      x = position.x -
        if spec.boldOffset == 0.0 then 0.0
        else case spec.align of
          AlignLeft -> 0.0
          AlignCenter -> width / 2.0
          AlignRight -> width
      baselineOffset = case spec.baseline of
        Alphabetic -> 0.0
        Middle -> metrics.capHeight / 2.0
        Top -> metrics.ascent
        Bottom -> negate metrics.descent
      y = position.y + baselineOffset - metrics.ascent
      color = deviceColor spec.color
    GPU.drawText { target, face, text: content, x, y, align, color }
    when (spec.boldOffset /= 0.0) $
      GPU.drawText { target, face, text: content, x: x + offset, y, align: 0, color }
