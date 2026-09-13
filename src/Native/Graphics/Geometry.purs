module Native.Graphics.Geometry
  ( identity
  , compose
  , translate
  , scale
  , apply
  , viewport
  , xScale
  , areaScale
  ) where

import Prelude hiding (identity)

import Data.Number as Number
import Native.Graphics.Types (Point, Rect, Transform)

identity :: Transform
identity = { a: 1.0, b: 0.0, c: 0.0, d: 1.0, tx: 0.0, ty: 0.0 }

compose :: Transform -> Transform -> Transform
compose m n =
  { a: m.a * n.a + m.c * n.b
  , b: m.b * n.a + m.d * n.b
  , c: m.a * n.c + m.c * n.d
  , d: m.b * n.c + m.d * n.d
  , tx: m.a * n.tx + m.c * n.ty + m.tx
  , ty: m.b * n.tx + m.d * n.ty + m.ty
  }

translate :: Number -> Number -> Transform
translate x y = identity { tx = x, ty = y }

scale :: Number -> Number -> Transform
scale x y = identity { a = x, d = y }

apply :: Transform -> Number -> Number -> Point
apply m x y = { x: m.a * x + m.c * y + m.tx, y: m.b * x + m.d * y + m.ty }

viewport :: Number -> Number -> Rect -> Transform
viewport width height rect =
  let
    zoom = min (width / rect.width) (height / rect.height)
  in
    { a: zoom
    , b: 0.0
    , c: 0.0
    , d: zoom
    , tx: (width - rect.width * zoom) / 2.0 - rect.x * zoom
    , ty: (height - rect.height * zoom) / 2.0 - rect.y * zoom
    }

xScale :: Transform -> Number
xScale m = Number.sqrt (m.a * m.a + m.b * m.b)

areaScale :: Transform -> Number
areaScale m = Number.sqrt (Number.abs (m.a * m.d - m.b * m.c))
