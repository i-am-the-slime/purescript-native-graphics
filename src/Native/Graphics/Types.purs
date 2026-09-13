module Native.Graphics.Types
  ( Image
  , Bytes
  , Color
  , Rect
  , Transform
  , Point
  , PathOp(..)
  , Path
  , LineJoin(..)
  , LineCap(..)
  , StrokeStyle
  , FillRule(..)
  , TextAlign(..)
  , TextBaseline(..)
  , TextSpec
  , CirclePatternSpec
  , LatticeSpec
  , LayerId(..)
  , BlendMode(..)
  , Layer
  , Composite
  , Command(..)
  , Drawing
  ) where

import Prelude

import Data.Maybe (Maybe)

-- | Premultiplied storage is a backend detail. Public color channels are
-- | straight-alpha values in the range zero to one.
type Color = { red :: Number, green :: Number, blue :: Number, alpha :: Number }

type Point = { x :: Number, y :: Number }

type Rect = { x :: Number, y :: Number, width :: Number, height :: Number }

-- | apply(x, y) = (a*x + c*y + tx, b*x + d*y + ty).
type Transform = { a :: Number, b :: Number, c :: Number, d :: Number, tx :: Number, ty :: Number }

data PathOp
  = MoveTo Number Number
  | LineTo Number Number
  | QuadTo Number Number Number Number
  | CubicTo Number Number Number Number Number Number
  | ClosePath

type Path = Array PathOp

data LineJoin = RoundJoin | BevelJoin | MiterJoin

data LineCap = ButtCap | RoundCap | SquareCap

type StrokeStyle = { width :: Number, join :: LineJoin, cap :: LineCap }

data FillRule = NonZero | EvenOdd

data TextAlign = AlignLeft | AlignCenter | AlignRight

data TextBaseline = Alphabetic | Middle | Top | Bottom

type TextSpec =
  { x :: Number
  , y :: Number
  , text :: String
  , font :: String
  , fontID :: Int
  , size :: Number
  , align :: TextAlign
  , baseline :: TextBaseline
  , boldOffset :: Number
  , color :: Color
  }

type CirclePatternSpec =
  { rect :: Rect
  , background :: Color
  , ink :: Color
  , tile :: Number
  , radius :: Number
  , originX :: Number
  , originY :: Number
  }

type LatticeSpec =
  { rect :: Rect
  , background :: Color
  , ink :: Color
  , pitchFraction :: Number
  , majorEvery :: Number
  , majorWidth :: Number
  , minorWidth :: Number
  , anchorX :: Number
  , anchorY :: Number
  }

newtype LayerId = LayerId Int

derive newtype instance eqLayerId :: Eq LayerId

derive newtype instance ordLayerId :: Ord LayerId

data BlendMode = SourceOver | Invert

type Layer = { global :: Boolean }

type Composite =
  { source :: LayerId
  , mask :: Maybe LayerId
  , invertMask :: Boolean
  , blend :: BlendMode
  }

data Command
  = FillPath Path Color
  | StrokePath Path Color StrokeStyle
  | FillStrokePath Path Color Color StrokeStyle
  | DrawText TextSpec
  | PushTransform Transform
  | PopTransform
  | PushClip Path FillRule
  | PopClip
  | PushAlpha Number
  | PopAlpha
  | PushLayer LayerId
  | PopLayer
  | Viewport Rect
  | Clear Color
  | CirclePattern CirclePatternSpec
  | PushBlur Number
  | PopBlur
  | LineLattice LatticeSpec

type Drawing =
  { commands :: Array Command
  , layers :: Array Layer
  , composite :: Array Composite
  , clear :: Color
  }

foreign import data Image :: Type

foreign import data Bytes :: Type
