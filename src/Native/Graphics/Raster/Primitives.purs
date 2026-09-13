module Native.Graphics.Raster.Primitives (buildPath, paint, rectangle, text, circlePattern, lattice, transparent, white) where

import Prelude
import Control.Monad.Rec.Class (Step(..), tailRecM)
import Data.Foldable (for_)
import Data.Int as Int
import Data.Number as Number
import Effect (Effect, forE)
import Native.Graphics.Font as Font
import Native.Graphics.Geometry as Geometry
import Native.Graphics.Raster.Native as N
import Native.Graphics.Types

transparent :: Color
transparent = { red: 0.0, green: 0.0, blue: 0.0, alpha: 0.0 }

white :: Color
white = { red: 1.0, green: 1.0, blue: 1.0, alpha: 1.0 }

buildPath :: Path -> Effect N.NativePath
buildPath ops = do
  path <- N.newPath
  for_ ops case _ of
    MoveTo x y -> N.moveTo { path, x, y }
    LineTo x y -> N.lineTo { path, x, y }
    QuadTo cx cy x y -> N.quadTo { path, cx, cy, x, y }
    CubicTo cx1 cy1 cx2 cy2 x y -> N.cubicTo { path, cx1, cy1, cx2, cy2, x, y }
    ClosePath -> N.closePath path
  pure path

rectangle :: Rect -> Path
rectangle r = [ MoveTo r.x r.y, LineTo (r.x + r.width) r.y, LineTo (r.x + r.width) (r.y + r.height), LineTo r.x (r.y + r.height), ClosePath ]

paint :: N.Surface -> Transform -> Path -> Color -> Color -> StrokeStyle -> FillRule -> Effect Unit
paint surface transform ops fill stroke style rule = do
  path <- buildPath ops
  let
    join = case style.join of
      RoundJoin -> 0
      BevelJoin -> 1
      MiterJoin -> 2
    cap = case style.cap of
      ButtCap -> 0
      RoundCap -> 1
      SquareCap -> 2
    evenOdd = case rule of
      EvenOdd -> true
      NonZero -> false
  N.paint { surface, transform, path, fill, stroke, width: style.width, join, cap, evenOdd }

text :: N.Surface -> Transform -> TextSpec -> Effect Unit
text surface transform spec = when (spec.text /= "") do
  font <- Font.resolveFamily spec.font
  shaped <- N.makeText { font, size: spec.size * 72.0 / 25.4, color: spec.color, text: spec.text }
  let
    offset = spec.size * spec.boldOffset
    width = shaped.width + offset
    x = spec.x - case spec.align of
      AlignLeft -> 0.0
      AlignCenter -> width / 2.0
      AlignRight -> width
    y = spec.y + case spec.baseline of
      Alphabetic -> 0.0
      Middle -> shaped.capHeight / 2.0
      Top -> shaped.ascent
      Bottom -> -shaped.descent
  N.paintText { surface, transform, line: shaped.line, x, y }
  when (offset /= 0.0) (N.paintText { surface, transform, line: shaped.line, x: x + offset, y })

circlePattern :: N.Surface -> Transform -> CirclePatternSpec -> Effect Unit
circlePattern surface transform spec = do
  let rect = rectangle spec.rect
  paint surface transform rect spec.background transparent { width: 0.0, join: RoundJoin, cap: ButtCap } NonZero
  when (spec.tile > 0.0 && spec.radius > 0.0) do
    let
      original = Geometry.compose (Geometry.compose transform (Geometry.translate spec.originX spec.originY)) (Geometry.scale spec.tile spec.tile)
      pitch = Geometry.xScale original
      cell = if pitch > 0.0 && pitch < 2.0 then Geometry.compose original (Geometry.scale (2.0 / pitch) (2.0 / pitch)) else original
      hatch bounds = do
        path <- N.newPath
        forE (Int.floor bounds.y0) (Int.ceil bounds.y1 + 1) \j ->
          forE (Int.floor bounds.x0) (Int.ceil bounds.x1 + 1) \i ->
            N.appendCircle { path, x: Int.toNumber i + 0.5, y: Int.toNumber j + 0.5, radius: spec.radius / spec.tile }
        pure path
    path <- buildPath rect
    N.paintHatch { surface, transform, path, cell, color: spec.ink, hatch }

lattice :: N.Surface -> Number -> Number -> LatticeSpec -> Effect Unit
lattice surface width height spec = do
  let
    pitch = spec.pitchFraction * min width height
    major = pitch * spec.majorEvery
  when (pitch > 0.0 && major > 0.0) do
    let
      remainder a b = a - Number.trunc (a / b) * b
      ox = remainder (width * spec.anchorX) major
      oy = remainder (height * spec.anchorY) major
      lines path spacing = do
        tailRecM
          ( \x ->
              if x > width then pure (Done unit)
              else do
                N.moveTo { path, x, y: 0.0 }
                N.lineTo { path, x, y: height }
                pure (Loop (x + spacing))
          )
          (ox - Number.ceil (ox / spacing) * spacing)
        tailRecM
          ( \y ->
              if y > height then pure (Done unit)
              else do
                N.moveTo { path, x: 0.0, y }
                N.lineTo { path, x: width, y }
                pure (Loop (y + spacing))
          )
          (oy - Number.ceil (oy / spacing) * spacing)
      draw spacing thickness = do
        path <- N.newPath
        lines path spacing
        N.paint { surface, transform: Geometry.identity, path, fill: transparent, stroke: spec.ink, width: thickness * 2.0, join: 0, cap: 0, evenOdd: false }
    paint surface Geometry.identity (rectangle { x: 0.0, y: 0.0, width, height }) spec.background transparent { width: 0.0, join: RoundJoin, cap: ButtCap } NonZero
    draw pitch spec.minorWidth
    draw major spec.majorWidth
