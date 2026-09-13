module Test.Native.Graphics.Metal.Geometry (main) where

import Prelude

import Data.Array as Array
import Data.Foldable (all, foldl)
import Effect (Effect)
import Effect.Exception (throw)
import Native.Graphics.Geometry as Transform
import Native.Graphics.Metal.Geometry as Geometry
import Native.Graphics.Types (FillRule(..), Path, PathOp(..))

rectPath :: Number -> Number -> Number -> Number -> Path
rectPath x y width height =
  [ MoveTo x y
  , LineTo (x + width) y
  , LineTo (x + width) (y + height)
  , LineTo x (y + height)
  , ClosePath
  ]

-- Check the actual triangles returned by the native tessellator and packed by
-- PureScript. A contour toggles the stencil once, not once per covering triangle.
drawContains :: Array Number -> Number -> Number -> Boolean
drawContains draw x y = go 0
  where
  cross ax ay bx by = (bx - ax) * (y - ay) - (by - ay) * (x - ax)
  go offset = case Array.slice offset (offset + 18) draw of
    [ ax, ay, _, _, _, _, bx, by, _, _, _, _, cx, cy, _, _, _, _ ] ->
      let
        ab = cross ax ay bx by
        bc = cross bx by cx cy
        ca = cross cx cy ax ay
        inside = (ab >= 0.0 && bc >= 0.0 && ca >= 0.0)
          || (ab <= 0.0 && bc <= 0.0 && ca <= 0.0)
      in
        inside || go (offset + 18)
    _ -> false

included :: Array (Array Number) -> Number -> Number -> Boolean
included draws x y = foldl (\parity draw -> if drawContains draw x y then not parity else parity) false draws

transparent :: Array Number -> Boolean
transparent draw = all identity $ Array.mapWithIndex (\index value -> mod index 6 < 2 || value == 0.0) draw

assert :: String -> Boolean -> Effect Unit
assert message condition = unless condition $ throw message

main :: Effect Unit
main = do
  let ring = rectPath 0.0 0.0 100.0 100.0 <> rectPath 25.0 25.0 50.0 50.0
  draws <- Geometry.clip Transform.identity ring EvenOdd
  assert "Even-odd clip must include the outer ring" $ included draws 10.0 12.0
  assert "Even-odd clip must exclude the nested contour" $ not $ included draws 50.0 52.0
  assert "Even-odd clip must exclude points outside both contours" $ not $ included draws 110.0 12.0
  assert "Clip vertices must keep the six-float transparent color layout" $
    all (\draw -> mod (Array.length draw) 18 == 0 && transparent draw) draws

  let transform = Transform.compose (Transform.translate 13.0 22.0) (Transform.scale 2.0 4.0)
  transformed <- Geometry.clip transform ring EvenOdd
  assert "Path coordinates must be transformed before tessellation" $ included transformed 33.0 70.0
  assert "Transformed nested contour must remain a hole" $ not $ included transformed 113.0 230.0
  assert "Transformed clip must not retain untransformed coordinates" $ not $ included transformed 10.0 12.0

  nonZero <- Geometry.clip Transform.identity ring NonZero
  assert "Nonzero clipping must submit all contours in a single stencil draw" $ Array.length nonZero == 1

  empty <- Geometry.clip Transform.identity [] EvenOdd
  assert "An empty even-odd path must not submit a stencil draw" $ Array.null empty
