module Native.Graphics.Metal.Vertices
  ( Vertices
  , Builder
  , empty
  , fromArray
  , toArray
  , length
  , concat
  , new
  , appendGlyphQuad
  , freeze
  ) where

import Prelude

import Control.Monad.ST (Region, ST)

foreign import data Vertices :: Type
foreign import data Builder :: Region -> Type

type role Builder nominal

foreign import empty :: Vertices
foreign import fromArray :: Array Number -> Vertices
foreign import toArray :: Vertices -> Array Number
-- | Number of floats, not vertices or bytes.
foreign import length :: Vertices -> Int
foreign import concat :: Array Vertices -> Vertices
-- | Initial capacity in floats.
foreign import new :: forall h. Int -> ST h (Builder h)
-- | Input: x0, y0, x1, y1, u0, v0, u1, v1, red, green, blue, alpha.
foreign import appendGlyphQuad :: forall h. Builder h -> Array Number -> ST h Unit
-- | Detach storage without copying; subsequent appends start a fresh buffer.
foreign import freeze :: forall h. Builder h -> ST h Vertices
