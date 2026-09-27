module Native.Graphics.Raster.Pixels (composite, mask, opacity, blur, boxRadii) where

import Prelude
import Data.Int as Int
import Data.Maybe (Maybe(..))
import Data.Number as Number
import Effect (Effect)
import Native.Graphics.Raster.Native as N
import Native.Graphics.Types (BlendMode(..), Image)

composite :: Image -> Image -> Maybe Image -> Boolean -> BlendMode -> Effect Unit
composite dst src stencil inverted blend = case stencil of
  Nothing -> N.compositeImage { dst, src, invert }
  Just image -> N.compositeMaskedImage { dst, src, mask: image, invertMask: inverted, invert }
  where
  invert = case blend of
    Invert -> true
    SourceOver -> false

mask :: Image -> Image -> Effect Unit
mask image stencil = N.maskImage { image, mask: stencil }

opacity :: Image -> Number -> Effect Unit
opacity image alpha = N.opacityImage { image, alpha }

boxRadii :: Number -> Array Int
boxRadii sigma =
  let
    ideal = Int.floor (Number.sqrt (4.0 * sigma * sigma + 1.0))
    lower = if mod ideal 2 == 0 then ideal - 1 else ideal
    l = Int.toNumber lower
    count = Int.round ((12.0 * sigma * sigma - 3.0 * l * l - 12.0 * l - 9.0) / (-4.0 * l - 4.0))
    radius i = if i < count then (lower - 1) `div` 2 else (lower + 1) `div` 2
  in
    [ radius 0, radius 1, radius 2 ]

blur :: Int -> Int -> Image -> Number -> Effect Unit
blur width height image sigma =
  when (width > 0 && height > 0 && sigma > 0.0) $
    N.blurImage { width, height, image, radii: boxRadii sigma }
