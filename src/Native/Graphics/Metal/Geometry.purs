module Native.Graphics.Metal.Geometry
  ( fill
  , stroke
  , fillStroke
  , clip
  , splitContours
  , roundedRect
  , capsule
  ) where

import Prelude

import Control.Monad.ST as ST
import Control.Monad.ST.Ref as STRef
import Data.Array as Array
import Data.Array.ST as STA
import Data.Maybe (Maybe(..))
import Data.Number as Number
import Data.Traversable (traverse)
import Effect (Effect, foreachE)
import Native.Graphics.Geometry as Geometry
import Native.Graphics.Metal.Vertices (Vertices)
import Native.Graphics.Metal.Vertices as Vertices
import Native.Graphics.Types (Color, FillRule(..), LineCap(..), LineJoin(..), Path, PathOp(..), Point, Rect, StrokeStyle, Transform)

buildPath :: Transform -> Path -> Effect NativePath
buildPath transform ops = do
  path <- newPath
  let point = Geometry.apply transform
  foreachE ops case _ of
    MoveTo x y -> moveTo { path, point: point x y }
    LineTo x y -> lineTo { path, point: point x y }
    QuadTo cx cy x y -> quadTo { path, control: point cx cy, point: point x y }
    CubicTo c1x c1y c2x c2y x y -> cubicTo
      { path, control1: point c1x c1y, control2: point c2x c2y, point: point x y }
    ClosePath -> closePath path
  pure path

fill :: Transform -> Number -> Path -> Color -> Effect Vertices
fill transform alpha path color = do
  native <- buildPath transform path
  tessellateFill { path: native, color: color { alpha = alpha * color.alpha } }

stroke :: Transform -> Number -> Path -> Color -> StrokeStyle -> Effect Vertices
stroke transform alpha path color style
  | style.width <= 0.0 = pure Vertices.empty
  | otherwise = do
      native <- buildPath transform path
      strokeNative transform alpha native color style

-- Fill and stroke share the same transformed path.
fillStroke :: Transform -> Number -> Path -> Color -> Color -> StrokeStyle -> Effect { fill :: Vertices, stroke :: Vertices }
fillStroke transform alpha path fillColor strokeColor style = do
  native <- buildPath transform path
  filled <- tessellateFill { path: native, color: fillColor { alpha = alpha * fillColor.alpha } }
  stroked <- if style.width <= 0.0 then pure Vertices.empty else strokeNative transform alpha native strokeColor style
  pure { fill: filled, stroke: stroked }

strokeNative :: Transform -> Number -> NativePath -> Color -> StrokeStyle -> Effect Vertices
strokeNative transform alpha native color style =
  tessellateStroke
    { path: native
    , color: color { alpha = alpha * color.alpha }
    , width: style.width * Geometry.xScale transform
    , join: case style.join of
        RoundJoin -> 0
        BevelJoin -> 1
        MiterJoin -> 2
    , cap: case style.cap of
        ButtCap -> 0
        RoundCap -> 1
        SquareCap -> 2
    }

-- Each MoveTo begins a separate stencil-toggle draw, including open contours.
splitContours :: Path -> Array Path
splitContours path = STA.run do
  out <- STA.new
  start <- STRef.new 0
  let count = Array.length path
  ST.for 0 count \index -> case Array.index path index of
    Just (MoveTo _ _) -> do
      first <- STRef.read start
      when (index > first) do
        void $ STA.push (Array.slice first index path) out
        void $ STRef.write index start
    _ -> pure unit
  first <- STRef.read start
  when (first < count) $ void $ STA.push (Array.slice first count path) out
  pure out

clip :: Transform -> Path -> FillRule -> Effect (Array Vertices)
clip transform path rule = do
  let transparent = { red: 0.0, green: 0.0, blue: 0.0, alpha: 0.0 }
  case rule of
    NonZero -> do
      vertices <- fill transform 1.0 path transparent
      pure [ vertices ]
    EvenOdd -> do
      draws <- traverse (\contour -> fill transform 1.0 contour transparent) (splitContours path)
      pure $ Array.filter (\vertices -> Vertices.length vertices > 0) draws

-- The SDF shader consumes position, local position, half extents, radius,
-- softness, thickness, then straight RGBA: thirteen floats per vertex.
sdfQuad :: Transform -> Number -> Number -> Number -> Number -> Number -> Number -> Color -> Array Number
sdfQuad transform alpha hx hy radius softness thickness color = STA.run do
  out <- STA.new
  let
    a = color.alpha * alpha
    pad = softness + thickness
    lx = hx + pad
    ly = hy + pad
    add x y = do
      let point = Geometry.apply transform x y
      void $ STA.push point.x out
      void $ STA.push point.y out
      void $ STA.push x out
      void $ STA.push y out
      void $ STA.push hx out
      void $ STA.push hy out
      void $ STA.push radius out
      void $ STA.push softness out
      void $ STA.push thickness out
      void $ STA.push color.red out
      void $ STA.push color.green out
      void $ STA.push color.blue out
      void $ STA.push a out
  add (-lx) (-ly)
  add lx (-ly)
  add lx ly
  add (-lx) (-ly)
  add lx ly
  add (-lx) ly
  pure out

roundedRect :: Transform -> Number -> Rect -> Number -> Number -> Number -> Color -> Array Number
roundedRect transform alpha rect radius softness thickness color =
  let
    hx = rect.width * 0.5
    hy = rect.height * 0.5
    frame = Geometry.compose transform $ Geometry.translate (rect.x + hx) (rect.y + hy)
  in
    sdfQuad frame alpha hx hy radius softness thickness color

capsule :: Transform -> Number -> Point -> Point -> Number -> Number -> Number -> Color -> Array Number
capsule transform alpha start end radius softness thickness color =
  let
    dx = end.x - start.x
    dy = end.y - start.y
    length = Number.sqrt (dx * dx + dy * dy)
    ux = if length > 0.000001 then dx / length else 1.0
    uy = if length > 0.000001 then dy / length else 0.0
    frame = Geometry.compose transform
      { a: ux, b: uy, c: -uy, d: ux, tx: (start.x + end.x) * 0.5, ty: (start.y + end.y) * 0.5 }
  in
    sdfQuad frame alpha (length * 0.5 + radius) radius radius softness thickness color

foreign import data NativePath :: Type

foreign import newPath :: Effect NativePath
foreign import moveTo :: { path :: NativePath, point :: Point } -> Effect Unit
foreign import lineTo :: { path :: NativePath, point :: Point } -> Effect Unit
foreign import quadTo :: { path :: NativePath, control :: Point, point :: Point } -> Effect Unit
foreign import cubicTo :: { path :: NativePath, control1 :: Point, control2 :: Point, point :: Point } -> Effect Unit
foreign import closePath :: NativePath -> Effect Unit
foreign import tessellateFill :: { path :: NativePath, color :: Color } -> Effect Vertices
foreign import tessellateStroke :: { path :: NativePath, color :: Color, width :: Number, join :: Int, cap :: Int } -> Effect Vertices
