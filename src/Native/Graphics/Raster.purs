module Native.Graphics.Raster (rasterize) where

import Prelude hiding (compose)
import Control.Monad.Rec.Class (Step(..), tailRec, tailRecM)
import Data.Array as Array
import Data.Foldable (for_)
import Data.Int as Int
import Data.List (List(..))
import Data.Maybe (Maybe(..))
import Data.Traversable (traverse)
import Effect (Effect)
import Native.Graphics.Geometry as Geometry
import Native.Graphics.Raster.Native as N
import Native.Graphics.Raster.Pixels as Pixels
import Native.Graphics.Raster.Primitives as P
import Native.Graphics.Types (Command(..), Drawing, FillRule(..), Image, LayerId(..), LineCap(..), LineJoin(..), Path, Transform)

type Raster = { width :: Int, height :: Int, layers :: Array N.Surface, drawing :: Drawing }
type State = { transform :: Transform, transforms :: List Transform, layers :: List Int, active :: Int, suppressed :: Int }

data Group = Clip Path FillRule | Blur Number | Alpha Number

initial :: Transform -> State
initial transform = { transform, transforms: Nil, layers: Nil, active: 0, suppressed: 0 }

newRaster :: Int -> Int -> Drawing -> Effect Raster
newRaster width height drawing = do
  layers <- traverse (\_ -> N.newSurface { width, height }) drawing.layers
  pure { width, height, layers, drawing }

compose :: Raster -> Effect Image
compose raster = do
  images <- traverse N.renderSurface raster.layers
  out <- N.newImage { width: raster.width, height: raster.height }
  for_ raster.drawing.composite \recipe -> do
    let LayerId source = recipe.source
    case Array.index images source of
      Nothing -> pure unit
      Just image -> do
        let stencil = recipe.mask >>= \(LayerId index) -> Array.index images index
        Pixels.composite out image stencil recipe.invertMask recipe.blend
  pure out

samePush :: Group -> Command -> Boolean
samePush (Clip _ _) (PushClip _ _) = true
samePush (Blur _) (PushBlur _) = true
samePush (Alpha _) (PushAlpha _) = true
samePush _ _ = false

samePop :: Group -> Command -> Boolean
samePop (Clip _ _) PopClip = true
samePop (Blur _) PopBlur = true
samePop (Alpha _) PopAlpha = true
samePop _ _ = false

matching :: Array Command -> Int -> Int -> Group -> Int
matching commands start limit group = tailRec step { index: start, depth: 0 }
  where
  step s = case Array.index commands s.index of
    Nothing -> Done limit
    Just command
      | s.index >= limit -> Done limit
      | samePush group command -> Loop { index: s.index + 1, depth: s.depth + 1 }
      | samePop group command && s.depth == 0 -> Done s.index
      | samePop group command -> Loop { index: s.index + 1, depth: s.depth - 1 }
      | otherwise -> Loop { index: s.index + 1, depth: s.depth }

transformBalance :: Array Command -> Int -> Int -> Int
transformBalance commands start limit = tailRec step { index: start, balance: 0 }
  where
  step s
    | s.index >= limit = Done s.balance
    | otherwise =
        let
          change = case Array.index commands s.index of
            Just (PushTransform _) -> 1
            Just PopTransform -> -1
            _ -> 0
        in
          Loop { index: s.index + 1, balance: s.balance + change }

run :: Raster -> Int -> Int -> State -> Effect Unit
run raster start limit state = tailRecM step { index: start, state }
  where
  commands = raster.drawing.commands
  noStroke = { width: 0.0, join: RoundJoin, cap: ButtCap }
  step cursor = case Array.index commands cursor.index of
    Nothing -> pure (Done unit)
    Just command
      | cursor.index >= limit -> pure (Done unit)
      | cursor.state.suppressed > 0 ->
          let
            delta = case command of
              PushTransform _ -> 1
              PopTransform -> -1
              _ -> 0
          in
            pure (Loop { index: cursor.index + 1, state: cursor.state { suppressed = cursor.state.suppressed + delta } })
      | otherwise -> do
          let
            s = cursor.state
            next updated = pure (Loop { index: cursor.index + 1, state: updated })
            active action = for_ (Array.index raster.layers s.active) action
            fill path color = active \surface -> P.paint surface s.transform path color P.transparent noStroke NonZero
            capture group = do
              let end = matching commands (cursor.index + 1) limit group
              let
                unbalanced = case group of
                  Clip _ _ -> transformBalance commands (cursor.index + 1) end < 0
                  _ -> false
              if unbalanced then next s
              else do
                sub <- newRaster raster.width raster.height raster.drawing
                run sub (cursor.index + 1) end (initial s.transform)
                image <- compose sub
                case group of
                  Blur amount -> do
                    let sigma = amount * Geometry.areaScale s.transform
                    when (sigma >= 0.5) (Pixels.blur raster.width raster.height image sigma)
                  Alpha amount -> Pixels.opacity image amount
                  Clip path rule -> do
                    stencil <- N.newSurface { width: raster.width, height: raster.height }
                    P.paint stencil s.transform path P.white P.transparent noStroke rule
                    N.renderSurface stencil >>= Pixels.mask image
                active \surface -> N.paintImage { surface, image }
                pure (Loop { index: end + 1, state: s })
          case command of
            FillPath path color -> fill path color *> next s
            StrokePath path color style -> do
              active \surface -> P.paint surface s.transform path P.transparent color style NonZero
              next s
            FillStrokePath path color stroke style -> do
              active \surface -> P.paint surface s.transform path color stroke style NonZero
              next s
            DrawText spec -> active (\surface -> P.text surface s.transform spec) *> next s
            PushTransform transform ->
              if transform.a * transform.d - transform.b * transform.c == 0.0 then next (s { suppressed = 1 })
              else next (s { transform = Geometry.compose s.transform transform, transforms = Cons s.transform s.transforms })
            PopTransform -> case s.transforms of
              Nil -> next s
              Cons previous rest -> next (s { transform = previous, transforms = rest })
            PushClip path rule -> capture (Clip path rule)
            PushBlur sigma -> capture (Blur sigma)
            PushAlpha alpha -> capture (Alpha alpha)
            PopClip -> next s
            PopBlur -> next s
            PopAlpha -> next s
            PushLayer (LayerId layer) -> next (s { active = layer, layers = Cons s.active s.layers })
            PopLayer -> case s.layers of
              Nil -> next s
              Cons previous rest -> next (s { active = previous, layers = rest })
            Viewport rect -> next (s { transform = Geometry.viewport (Int.toNumber raster.width) (Int.toNumber raster.height) rect })
            Clear color -> do
              active \surface -> P.paint surface Geometry.identity (P.rectangle { x: 0.0, y: 0.0, width: Int.toNumber raster.width, height: Int.toNumber raster.height }) color P.transparent noStroke NonZero
              next s
            CirclePattern spec -> active (\surface -> P.circlePattern surface s.transform spec) *> next s
            LineLattice spec -> active (\surface -> P.lattice surface (Int.toNumber raster.width) (Int.toNumber raster.height) spec) *> next s

rasterize :: Int -> Int -> Drawing -> Effect Image
rasterize width height drawing = N.critical do
  raster <- newRaster width height drawing
  run raster 0 (Array.length drawing.commands) (initial Geometry.identity)
  compose raster
