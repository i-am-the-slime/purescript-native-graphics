module Native.Graphics.Metal (create) where

import Prelude hiding (top)

import Data.Array as Array
import Data.Foldable (traverse_)
import Data.Int as Int
import Data.List (List(..))
import Data.List as List
import Data.Maybe (Maybe(..), fromMaybe)
import Data.Number as Number
import Data.Set as Set
import Data.Traversable (traverse)
import Effect (Effect, foreachE)
import Effect.Exception (catchException, throw, throwException)
import Effect.Ref as Ref
import Native.Graphics.Geometry as Matrix
import Native.Graphics.Metal.Geometry as Geometry
import Native.Graphics.Metal.Primitives as GPU
import Native.Graphics.Metal.Text as Text
import Native.Graphics.Metal.Vertices (Vertices)
import Native.Graphics.Metal.Vertices as Vertices
import Native.Graphics.Types (BlendMode(..), Color, Command(..), Drawing, FillRule(..), LayerId(..), Rect, Transform)

type Targets = { main :: Array GPU.Target, capture :: Array GPU.Target, scratchA :: GPU.Target, scratchB :: GPU.Target, width :: Int, height :: Int }
type BufferState = { buffer :: GPU.Buffer, capacity :: Int, offset :: Int }
type Clip = { draws :: Array Vertices, evenOdd :: Boolean }
type State =
  { transforms :: List Transform
  , alphas :: List Number
  , layers :: List Int
  , clips :: Array Clip
  , blurs :: List Number
  , blurLayer :: Int
  , viewportApplied :: Boolean
  , batch :: List Vertices
  }

type PassState = { mainVisited :: Set.Set Int, captureVisited :: Set.Set Int, current :: Maybe { layer :: Int, capture :: Boolean } }

initial :: Transform -> Boolean -> State
initial transform viewportApplied =
  { transforms: if viewportApplied then Cons transform (Cons Matrix.identity Nil) else Cons Matrix.identity Nil
  , alphas: Cons 1.0 Nil
  , layers: Cons 0 Nil
  , clips: []
  , blurs: Nil
  , blurLayer: 0
  , viewportApplied
  , batch: Nil
  }

top :: forall a. a -> List a -> a
top fallback = fromMaybe fallback <<< List.head

pop :: forall a. List a -> List a
pop values = case values of
  Cons _ rest@(Cons _ _) -> rest
  _ -> values

rgba :: Number -> Color -> Array Number
rgba alpha color = [ color.red, color.green, color.blue, color.alpha * alpha ]

premultiplied :: Color -> Array Number
premultiplied color = [ color.red * color.alpha, color.green * color.alpha, color.blue * color.alpha, color.alpha ]

transparent :: Array Number
transparent = [ 0.0, 0.0, 0.0, 0.0 ]

layerIndex :: LayerId -> Int
layerIndex (LayerId value) = value

recipe :: Drawing -> Effect (Array Int)
recipe drawing = do
  let count = Array.length drawing.layers
  when (count < 1 || count > 31) $ throw "Metal requires 1..31 layers"
  when (Array.null drawing.composite || Array.length drawing.composite > 256) $ throw "Metal requires 1..256 composition steps"
  Array.concat <$> traverse
    ( \op -> do
        let source = layerIndex op.source
        let mask = maybeLayer op.mask
        when (source < 0 || source >= count || mask < -1 || mask >= count) $ throw "Invalid Metal composition layer"
        pure
          [ source
          , mask
          , if op.invertMask then 1 else 0
          , case op.blend of
              SourceOver -> 0
              Invert -> 1
          ]
    )
    drawing.composite
  where
  maybeLayer Nothing = -1
  maybeLayer (Just value) = layerIndex value

releaseTargets :: Targets -> Effect Unit
releaseTargets resources = do
  traverse_ GPU.releaseTarget resources.main
  traverse_ GPU.releaseTarget resources.capture
  unless (Array.null resources.capture) do
    GPU.releaseTarget resources.scratchA
    GPU.releaseTarget resources.scratchB

-- | Explicitly owned reusable targets, vertex storage and font atlas cache.
-- | The native window close notification releases the closure's GPU resources.
create :: Effect (Drawing -> Drawing -> Number -> Number -> Int -> Int -> Effect Unit)
create = do
  resourcesRef <- Ref.new Nothing
  bufferRef <- Ref.new Nothing
  text <- Text.create
  GPU.onClose do
    Ref.read resourcesRef >>= traverse_ releaseTargets
    Ref.write Nothing resourcesRef
    Ref.read bufferRef >>= traverse_ (GPU.releaseBuffer <<< _.buffer)
    Ref.write Nothing bufferRef
  let
    emit pipeline stride uniform vertices = do
      let bytes = Vertices.length vertices * 4
      unless (bytes == 0) do
        existing <- Ref.read bufferRef
        let oldOffset = fromMaybe 0 (_.offset <$> existing)
        let needed = oldOffset + bytes
        storage <- case existing of
          Just value | needed <= value.capacity -> pure value
          _ -> do
            let capacity = grow (max 4096 (fromMaybe 0 (_.capacity <$> existing))) needed
            buffer <- GPU.newBuffer capacity
            traverse_ (GPU.releaseBuffer <<< _.buffer) existing
            pure { buffer, capacity, offset: 0 }
        GPU.draw { buffer: storage.buffer, offset: storage.offset, vertices, pipeline, stride, uniform }
        let offset = ((storage.offset + bytes + 15) `div` 16) * 16
        Ref.write (Just (storage { offset = offset })) bufferRef
    grow capacity needed = if capacity >= needed then capacity else grow (capacity * 2) needed
    targets width height layers needsBlur = do
      old <- Ref.read resourcesRef
      case old of
        Just value | value.width == width && value.height == height && Array.length value.main == Array.length layers && (not needsBlur || not (Array.null value.capture)) -> pure value
        _ -> do
          traverse_ releaseTargets old
          main <- traverse (const (GPU.newTarget { width, height, multisample: true })) layers
          base <- case Array.head main of
            Just value -> pure value
            Nothing -> throw "Metal requires a base target"
          capture <- if needsBlur then traverse (const (GPU.newTarget { width, height, multisample: true })) layers else pure []
          -- Drawable composition ignores its destination argument. Without
          -- blur, use the base handle rather than allocate unused scratch.
          scratchA <- if needsBlur then GPU.newTarget { width, height, multisample: false } else pure base
          scratchB <- if needsBlur then GPU.newTarget { width, height, multisample: false } else pure base
          let value = { main, capture, scratchA, scratchB, width, height }
          Ref.write (Just value) resourcesRef
          pure value
    finishFrame action = catchException (\error -> GPU.endFrame *> throwException error) (action <* GPU.endFrame)
  pure \frame overlay viewportWidth viewportHeight frameWidth frameHeight -> do
    ops <- recipe frame
    when (viewportWidth <= 0.0 || viewportHeight <= 0.0 || frameWidth <= 0 || frameHeight <= 0) $ throw "Metal requires positive viewport and authored dimensions"
    begun <- GPU.beginFrame
    when begun $ finishFrame do
      scale <- max 1.0 <$> GPU.backingScale
      let width = viewportWidth
      let height = viewportHeight
      let
        isBlur = case _ of
          PushBlur _ -> true
          _ -> false
      resources <- targets (Int.ceil (width * scale)) (Int.ceil (height * scale)) frame.layers
        (Array.any isBlur frame.commands || Array.any isBlur overlay.commands)
      Ref.modify_ (map (_ { offset = 0 })) bufferRef
      passRef <- Ref.new { mainVisited: Set.empty, captureVisited: Set.empty, current: Nothing }
      let
        setDepth depth
          | depth <= 0 = GPU.stencil { state: 0, depth: 0, reference: 0 }
          | depth > 8 = GPU.stencil { state: 1, depth: 0, reference: 0 }
          | otherwise = GPU.stencil { state: 5, depth, reference: Int.floor (Number.pow 2.0 (Int.toNumber depth)) - 1 }
        install clip depth pushing = do
          when (depth <= 8) do
            let bit = Int.floor (Number.pow 2.0 (Int.toNumber (depth - 1)))
            let state = if clip.evenOdd then 3 else if pushing then 2 else 4
            let reference = if clip.evenOdd then bit - 1 else bit * 2 - 1
            foreachE clip.draws \vertices -> do
              GPU.stencil { state, depth, reference }
              emit 6 6 [] vertices
          setDepth (if pushing then depth else depth - 1)
        reinstall clips = foreachE (Array.mapWithIndex (\i clip -> { depth: i + 1, clip }) clips) \entry -> install entry.clip entry.depth true
        activate state force layer = do
          when (layer < 0 || layer >= Array.length frame.layers) $ throw "Invalid Metal drawing layer"
          pass <- Ref.read passRef
          let global = fromMaybe false (_.global <$> Array.index frame.layers layer)
          let capture = not (List.null state.blurs) && not global
          let selected = { layer, capture }
          unless (not force && pass.current == Just selected) do
            let visited = if capture then pass.captureVisited else pass.mainVisited
            let collection = if capture then resources.capture else resources.main
            case Array.index collection layer of
              Nothing -> throw "Missing Metal layer target"
              Just target -> do
                GPU.beginPass { target, clear: not (Set.member layer visited), color: if layer == 0 && not capture then premultiplied frame.clear else transparent }
                Ref.write (if capture then pass { captureVisited = Set.insert layer visited, current = Just selected } else pass { mainVisited = Set.insert layer visited, current = Just selected }) passRef
                reinstall state.clips
        clearUnvisited capture = do
          pass <- Ref.read passRef
          let visited = if capture then pass.captureVisited else pass.mainVisited
          foreachE (Array.mapWithIndex (\layer target -> { layer, target }) (if capture then resources.capture else resources.main)) \entry ->
            unless (Set.member entry.layer visited) $ GPU.beginPass { target: entry.target, clear: true, color: if entry.layer == 0 && not capture then premultiplied frame.clear else transparent }
          GPU.endPass
          Ref.modify_ (_ { current = Nothing }) passRef
        render drawing transform viewportApplied = do
          stateRef <- Ref.new (initial transform viewportApplied)
          let
            flush = do
              state <- Ref.read stateRef
              emit 0 6 [] (Vertices.concat (Array.fromFoldable (List.reverse state.batch)))
              Ref.modify_ (_ { batch = Nil }) stateRef
            enqueue vertices = unless (Vertices.length vertices == 0) $ Ref.modify_ (\state -> state { batch = Cons vertices state.batch }) stateRef
            command operation = do
              state <- Ref.read stateRef
              let matrix = top Matrix.identity state.transforms
              let alpha = top 1.0 state.alphas
              case operation of
                FillPath path color -> Geometry.fill matrix alpha path color >>= enqueue
                StrokePath path color style -> Geometry.stroke matrix alpha path color style >>= enqueue
                FillStrokePath path fillColor strokeColor style -> do
                  Geometry.fill matrix alpha path fillColor >>= enqueue
                  Geometry.stroke matrix alpha path strokeColor style >>= enqueue
                DrawText spec -> do
                  flush
                  run <- text matrix alpha (spec { size = spec.size * Matrix.xScale matrix })
                  emit 5 8 [ run.screenPxRange ] run.vertices
                PushTransform next -> Ref.modify_ (_ { transforms = Cons (Matrix.compose matrix next) state.transforms }) stateRef
                PopTransform -> Ref.modify_ (_ { transforms = pop state.transforms }) stateRef
                PushAlpha value -> Ref.modify_ (_ { alphas = Cons (alpha * value) state.alphas }) stateRef
                PopAlpha -> Ref.modify_ (_ { alphas = pop state.alphas }) stateRef
                PushClip path rule -> do
                  flush
                  draws <- Geometry.clip matrix path rule
                  let
                    clip =
                      { draws
                      , evenOdd: case rule of
                          EvenOdd -> true
                          NonZero -> false
                      }
                  let clips = Array.snoc state.clips clip
                  Ref.modify_ (_ { clips = clips }) stateRef
                  install clip (Array.length clips) true
                PopClip -> do
                  flush
                  case Array.unsnoc state.clips of
                    Nothing -> setDepth 0
                    Just { init: clips, last: clip } -> do
                      install clip (Array.length state.clips) false
                      Ref.modify_ (_ { clips = clips }) stateRef
                PushLayer id -> do
                  flush
                  let layer = layerIndex id
                  Ref.modify_ (_ { layers = Cons layer state.layers }) stateRef
                  activate state false layer
                PopLayer -> do
                  flush
                  let layers = pop state.layers
                  Ref.modify_ (_ { layers = layers }) stateRef
                  activate state false (top 0 layers)
                Viewport rect -> do
                  let transforms = if state.viewportApplied then pop state.transforms else state.transforms
                  let next = Matrix.compose (top Matrix.identity transforms) (Matrix.viewport width height rect)
                  Ref.modify_ (_ { transforms = Cons next transforms, viewportApplied = true }) stateRef
                Clear _ -> pure unit
                CirclePattern spec -> do
                  flush
                  emit 3 4 ([ spec.tile, spec.radius, spec.originX, spec.originY ] <> rgba alpha spec.background <> rgba alpha spec.ink) (quad matrix spec.rect false)
                LineLattice spec -> do
                  flush
                  let pitch = spec.pitchFraction * min width height
                  let major = pitch * spec.majorEvery
                  when (pitch > 0.0 && major > 0.0) do
                    let remainder value = value - Number.trunc (value / major) * major
                    let uniform = [ pitch, spec.majorEvery, spec.majorWidth, spec.minorWidth, remainder (spec.anchorX * width), remainder (spec.anchorY * height), 0.0, 0.0 ] <> rgba alpha spec.background <> rgba alpha spec.ink
                    emit 4 4 uniform (quad matrix spec.rect true)
                PushBlur sigma -> do
                  let next = state { blurs = Cons (sigma * Matrix.xScale matrix) state.blurs, blurLayer = if List.null state.blurs then top 0 state.layers else state.blurLayer }
                  when (List.null state.blurs) do
                    flush
                    Ref.modify_ (_ { captureVisited = Set.empty, current = Nothing }) passRef
                    activate next true (top 0 state.layers)
                  Ref.modify_ (_ { blurs = next.blurs, blurLayer = next.blurLayer }) stateRef
                PopBlur -> case state.blurs of
                  Nil -> pure unit
                  Cons sigma rest -> do
                    Ref.modify_ (_ { blurs = rest }) stateRef
                    when (List.null rest) do
                      flush
                      clearUnvisited true
                      GPU.compose { sources: resources.capture, ops, destination: resources.scratchA, drawable: false }
                      let sigmaPixels = sigma * scale
                      when (sigmaPixels >= 0.25) do
                        GPU.filter { source: resources.scratchA, destination: resources.scratchB, uniform: [ 1.0 / Int.toNumber resources.width, 0.0, sigmaPixels, 0.0 ] }
                        GPU.filter { source: resources.scratchB, destination: resources.scratchA, uniform: [ 0.0, 1.0 / Int.toNumber resources.height, sigmaPixels, 0.0 ] }
                      activate (state { blurs = rest, clips = [] }) true state.blurLayer
                      GPU.blit resources.scratchA
                      reinstall state.clips
          activate (initial transform viewportApplied) true 0
          foreachE drawing.commands command
          flush
      render frame Matrix.identity false
      render overlay (Matrix.viewport width height { x: 0.0, y: 0.0, width: Int.toNumber frameWidth, height: Int.toNumber frameHeight }) true
      clearUnvisited false
      GPU.compose { sources: resources.main, ops, destination: resources.scratchA, drawable: true }

quad :: Transform -> Rect -> Boolean -> Vertices
quad matrix rect screenLocal =
  let
    vertex x y = let point = Matrix.apply matrix x y in [ point.x, point.y, if screenLocal then point.x else x, if screenLocal then point.y else y ]
    a = vertex rect.x rect.y
    b = vertex (rect.x + rect.width) rect.y
    c = vertex (rect.x + rect.width) (rect.y + rect.height)
    d = vertex rect.x (rect.y + rect.height)
  in
    Vertices.fromArray (a <> b <> c <> a <> c <> d)
