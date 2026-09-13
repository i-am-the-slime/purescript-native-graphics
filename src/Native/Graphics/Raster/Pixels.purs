module Native.Graphics.Raster.Pixels (composite, mask, opacity, blur, boxRadii) where

import Prelude
import Data.Foldable (for_)
import Data.Int as Int
import Data.Maybe (Maybe(..))
import Data.Number as Number
import Effect (Effect, forE)
import Effect.Ref as Ref
import Native.Graphics.Raster.Native as N
import Native.Graphics.Types (BlendMode(..), Image)

composite :: Image -> Image -> Maybe Image -> Boolean -> BlendMode -> Effect Unit
composite dst src stencil inverted blend =
  forE 0 (N.imageLength dst `div` 4) \pixel -> do
    let i = pixel * 4
    coverage <- case stencil of
      Nothing -> pure 255
      Just image -> do
        a <- N.readByte image (i + 3)
        pure (if inverted then 255 - a else a)
    sourceAlpha <- N.readByte src (i + 3)
    let alpha = (sourceAlpha * coverage) `div` 255
    when (alpha /= 0) do
      forE 0 3 \channel -> do
        d <- N.readByte dst (i + channel)
        value <- case blend of
          Invert -> pure ((d * (255 - alpha) + (255 - d) * alpha) `div` 255)
          SourceOver -> do
            s <- N.readByte src (i + channel)
            pure (min 255 ((s * coverage) `div` 255 + (d * (255 - alpha)) `div` 255))
        N.writeByte dst (i + channel) value
      d <- N.readByte dst (i + 3)
      N.writeByte dst (i + 3) (min 255 (alpha + (d * (255 - alpha)) `div` 255))

mask :: Image -> Image -> Effect Unit
mask image stencil = when (N.imageLength image == N.imageLength stencil) do
  forE 0 (N.imageLength image `div` 4) \pixel -> do
    let i = pixel * 4
    alpha <- N.readByte stencil (i + 3)
    forE 0 4 \channel -> do
      value <- N.readByte image (i + channel)
      N.writeByte image (i + channel) ((value * alpha) `div` 255)

opacity :: Image -> Number -> Effect Unit
opacity image alpha = forE 0 (N.imageLength image) \i -> do
  value <- N.readByte image i
  N.writeByte image i (Int.floor (Int.toNumber value * alpha))

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

-- The buffer is native float32 storage; traversal and edge extension are PS.
-- One scalar running sum per channel avoids boxed per-pixel collections.
boxPass :: N.FloatBuffer -> N.FloatBuffer -> Int -> Int -> Int -> Int -> Int -> Effect Unit
boxPass src dst lines length lineStride step radius =
  if radius <= 0 then N.copyFloats src dst
  else
    let
      norm = N.float32 (1.0 / N.float32 (Int.toNumber (2 * radius + 1)))
    in
      forE 0 lines \line -> forE 0 4 \channel -> do
        let base = line * lineStride + channel
        first <- N.readFloat src base
        sum <- Ref.new (N.float32 (first * N.float32 (Int.toNumber (radius + 1))))
        forE 1 (radius + 1) \x -> do
          value <- N.readFloat src (base + min x (length - 1) * step)
          Ref.modify_ (\total -> N.float32 (total + value)) sum
        forE 0 length \x -> do
          value <- Ref.read sum
          N.writeFloat dst (base + x * step) (value * norm)
          add <- N.readFloat src (base + min (x + radius + 1) (length - 1) * step)
          drop <- N.readFloat src (base + max (x - radius) 0 * step)
          Ref.modify_ (\total -> N.float32 (total + N.float32 (add - drop))) sum

blur :: Int -> Int -> Image -> Number -> Effect Unit
blur width height image sigma = when (width > 0 && height > 0 && sigma > 0.0) do
  let length = width * height * 4
  src <- N.newFloats length
  tmp <- N.newFloats length
  forE 0 length \i -> N.readByte image i >>= (N.writeFloat src i <<< Int.toNumber)
  for_ (boxRadii sigma) \radius -> do
    boxPass src tmp height width (width * 4) 4 radius
    boxPass tmp src width height 4 (width * 4) radius
  forE 0 length \i -> do
    value <- N.readFloat src i
    N.writeByte image i (Int.floor (value + 0.5))
