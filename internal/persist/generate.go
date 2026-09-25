package persist

import "math"

// Block IDs must match aarukanclient/world/block_types.gd
const (
	BlockAir       uint16 = 0
	BlockGrass     uint16 = 1
	BlockDirt      uint16 = 2
	BlockStone     uint16 = 3
	BlockCobble    uint16 = 4
	BlockOakLog    uint16 = 5
	BlockOakPlanks uint16 = 6
	BlockLeaves    uint16 = 7
	BlockSand      uint16 = 8
	BlockWater     uint16 = 10
	BlockBedrock   uint16 = 16
)

const lakeSurface = 22

// terrainSample is the solid surface, water top (-1 if dry), and mount factor.
type terrainSample struct {
	solid int
	water int
	mount float64
	lake  float64
}

// GenerateChunk fills a new column with a deterministic heightmap + spawn plaza.
func GenerateChunk(coord ChunkCoord) *Chunk {
	c := EmptyChunk(coord)
	ox := int(coord.X) * ChunkSize
	oz := int(coord.Z) * ChunkSize
	for lz := 0; lz < ChunkSize; lz++ {
		for lx := 0; lx < ChunkSize; lx++ {
			wx := ox + lx
			wz := oz + lz
			s := sampleTerrain(wx, wz)
			for y := 0; y < ChunkHeight; y++ {
				c.Blocks[BlockIndex(lx, y, lz)] = columnBlock(wx, y, wz, s.solid, s.water, s.mount)
			}
		}
	}
	// Include neighbor trunks so canopies aren't clipped at chunk borders.
	const treeMargin = 2
	for wz := oz - treeMargin; wz < oz+ChunkSize+treeMargin; wz++ {
		for wx := ox - treeMargin; wx < ox+ChunkSize+treeMargin; wx++ {
			s := sampleTerrain(wx, wz)
			if s.water >= s.solid || s.mount > 18.0 {
				continue
			}
			if s.solid+8 >= ChunkHeight || !shouldPlantTree(wx, wz) {
				continue
			}
			plantTreeInChunk(c, ox, oz, wx, s.solid+1, wz)
		}
	}
	if coord.X == 0 && coord.Z == 0 {
		carveSpawnPlaza(c)
	}
	SealBedrock(c)
	return c
}

// SealBedrock forces an unbreakable floor at y=0 (also repairs older saves).
func SealBedrock(c *Chunk) {
	if c == nil {
		return
	}
	for lz := 0; lz < ChunkSize; lz++ {
		for lx := 0; lx < ChunkSize; lx++ {
			c.Blocks[BlockIndex(lx, 0, lz)] = BlockBedrock
		}
	}
}

// HeightAt is the solid surface Y for world (x, z).
func HeightAt(x, z int) int {
	return sampleTerrain(x, z).solid
}

// WaterAt is the water surface Y, or -1 when the column is dry.
func WaterAt(x, z int) int {
	return sampleTerrain(x, z).water
}

func smoothstep(edge0, edge1, x float64) float64 {
	t := (x - edge0) / (edge1 - edge0)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

// bankHeight is uncarved land rim height (before river/canyon cuts).
func bankHeight(x, z int) float64 {
	plains := smoothNoise(x, z, 90.0)
	detail := smoothNoise(x+19, z-7, 35.0)
	hills := smoothNoise(x-11, z+29, 55.0)
	rangeRaw := smoothNoise(x-41, z+23, 240.0)
	rangeMask := smoothstep(-0.35, 0.12, rangeRaw)
	ridgeA := 1.0 - math.Abs(smoothNoise(x+7, z-11, 150.0))
	ridgeB := 1.0 - math.Abs(smoothNoise(x-29, z+17, 130.0))
	ridge := math.Max(ridgeA*ridgeA, ridgeB*ridgeB*0.7)
	ridge = math.Pow(ridge, 1.15)
	highland := rangeMask * ridge
	slope := smoothstep(0.08, 0.55, highland)
	mount := rangeMask * ridge * (0.4+(1.0-0.4)*slope) * 38.0
	lakeRaw := smoothNoise(x+61, z-47, 320.0)
	lake := smoothstep(0.38, 0.68, lakeRaw)
	base := 22.0 + plains*7.0 + detail*2.5 + hills*6.0
	return base + mount - lake*10.0
}

// sampleTerrain mirrors aarukanclient ChunkTerrain.terrain_sample.
func sampleTerrain(x, z int) terrainSample {
	fx := float64(x)
	fz := float64(z)

	plains := smoothNoise(x, z, 90.0)
	detail := smoothNoise(x+19, z-7, 35.0)
	hills := smoothNoise(x-11, z+29, 55.0)

	rangeRaw := smoothNoise(x-41, z+23, 240.0)
	rangeMask := smoothstep(-0.35, 0.12, rangeRaw)

	ridgeA := 1.0 - math.Abs(smoothNoise(x+7, z-11, 150.0))
	ridgeB := 1.0 - math.Abs(smoothNoise(x-29, z+17, 130.0))
	ridge := math.Max(ridgeA*ridgeA, ridgeB*ridgeB*0.7)
	ridge = math.Pow(ridge, 1.15)

	highland := rangeMask * ridge
	slope := smoothstep(0.08, 0.55, highland)
	mount := rangeMask * ridge * (0.4+(1.0-0.4)*slope) * 38.0

	const warpAmp = 32.0
	wx := fx + smoothNoise(x+3, z-5, 80.0)*warpAmp
	wz := fz + smoothNoise(x+53, z-35, 80.0)*warpAmp
	iwx := int(math.Round(wx))
	iwz := int(math.Round(wz))

	canyonN := math.Abs(smoothNoise(iwx-13, iwz+31, 55.0))
	const canyonW = 0.11
	canyonT := math.Max(0, canyonW-canyonN) / canyonW
	canyonCarve := canyonT * canyonT * 22.0 * rangeMask

	lakeRaw := smoothNoise(x+61, z-47, 320.0)
	lake := smoothstep(0.38, 0.68, lakeRaw)
	lakeCarve := lake * 10.0

	riverN := math.Abs(smoothNoise(iwx+101, iwz-67, 110.0))
	riverW := 0.032 + 0.028*(1.0-rangeMask) + 0.02*lake
	riverT := math.Max(0, riverW-riverN) / math.Max(riverW, 0.001)
	riverDepth := 3.5 + 2.0*rangeMask + 2.5*lake
	riverCarve := riverT * riverT * riverDepth

	base := 22.0 + plains*7.0 + detail*2.5 + hills*6.0
	bankF := base + mount - lakeCarve

	const neighborR = 6
	const drainR = 16
	bankSum := 0.0
	spill := bankF
	farSpill := bankF
	nCount := 0
	for _, oz := range []int{-neighborR, 0, neighborR} {
		for _, ox := range []int{-neighborR, 0, neighborR} {
			if ox == 0 && oz == 0 {
				continue
			}
			nb := bankHeight(x+ox, z+oz)
			bankSum += nb
			if nb < spill {
				spill = nb
			}
			nCount++
		}
	}
	for _, oz := range []int{-drainR, 0, drainR} {
		for _, ox := range []int{-drainR, 0, drainR} {
			if ox == 0 && oz == 0 {
				continue
			}
			nb := bankHeight(x+ox, z+oz)
			if nb < farSpill {
				farSpill = nb
			}
		}
	}
	valleyDepth := (bankSum / float64(nCount)) - bankF
	valleyW := smoothstep(0.8, 5.0, valleyDepth)

	lakeProtect := 1.0 - smoothstep(0.25, 0.55, lake)
	riverCarve *= valleyW * lakeProtect
	canyonCarve *= (0.35 + (1.0-0.35)*valleyW) * lakeProtect

	solidF := bankF - canyonCarve - riverCarve
	solidY := int(math.Round(solidF))

	waterY := -1
	if lake > 0.2 {
		floorY := lakeSurface - 2 - int(math.Round(lake*6.0))
		t := lake * 1.15
		if t > 1 {
			t = 1
		}
		solidY = int(math.Round(float64(solidY)*(1-t) + float64(floorY)*t))
		if solidY > floorY+1 {
			solidY = floorY + 1
		}
		if solidY < lakeSurface {
			waterY = lakeSurface
		}
	}

	if waterY < 0 && lake > 0.12 && solidY < lakeSurface && bankF <= float64(lakeSurface)+3.0 {
		waterY = lakeSurface
		if solidY > lakeSurface-2 {
			solidY = lakeSurface - 2
		}
	}

	const maxChannelDepth = 3
	if waterY < 0 && (riverCarve > 1.2 || canyonCarve > 6.0) && valleyW > 0.25 {
		channelSurface := int(math.Round(spill)) - 1
		bankSurface := int(math.Round(bankF)) - 1
		if bankSurface < channelSurface {
			channelSurface = bankSurface
		}
		if bankF-farSpill > 4.0 {
			drained := int(math.Round(farSpill)) + 1
			if drained < channelSurface {
				channelSurface = drained
			}
		}
		if solidY+maxChannelDepth < channelSurface {
			channelSurface = solidY + maxChannelDepth
		}
		if channelSurface > solidY {
			waterY = channelSurface
		}
	}

	if waterY >= 0 && waterY <= solidY {
		if lake > 0.2 {
			if solidY > lakeSurface-2 {
				solidY = lakeSurface - 2
			}
			waterY = lakeSurface
		} else {
			waterY = -1
		}
	}

	if solidY < 4 {
		solidY = 4
	}
	if solidY > ChunkHeight-8 {
		solidY = ChunkHeight - 8
	}
	if waterY >= 0 {
		if waterY <= solidY {
			waterY = -1
		} else if waterY > ChunkHeight-2 {
			waterY = ChunkHeight - 2
		}
	}

	return terrainSample{solid: solidY, water: waterY, mount: mount, lake: lake}
}

func columnBlock(x, y, z, surface, water int, mount float64) uint16 {
	if y == 0 {
		return BlockBedrock
	}
	if water >= 0 && y > surface && y <= water {
		return BlockWater
	}
	if y > surface {
		return BlockAir
	}
	if y == surface {
		if water >= surface || (water >= 0 && surface <= water+1) {
			return BlockSand
		}
		if mount > 28.0 {
			return BlockStone
		}
		if mount > 14.0 {
			if (x+z)%3 == 0 {
				return BlockStone
			}
			return BlockDirt
		}
		if surface <= 14 {
			return BlockSand
		}
		return BlockGrass
	}
	if y >= surface-3 {
		if mount < 28.0 {
			return BlockDirt
		}
		return BlockStone
	}
	if y <= 2 {
		return BlockStone
	}
	if y < surface-8 && (x+z+y)%11 == 0 {
		return BlockCobble
	}
	return BlockStone
}

func shouldPlantTree(wx, wz int) bool {
	if smoothNoise(wx+91, wz-13, 8.0) <= 0.55 {
		return false
	}
	h := (wx*73856093 ^ wz*19349663) & 0x7fffffff
	return h%11 == 0
}

func plantTreeInChunk(c *Chunk, ox, oz, wx, baseY, wz int) {
	const trunkH = 6
	lx := wx - ox
	lz := wz - oz
	if lx >= 0 && lx < ChunkSize && lz >= 0 && lz < ChunkSize {
		for i := 0; i < trunkH; i++ {
			y := baseY + i
			if y >= ChunkHeight {
				break
			}
			c.Blocks[BlockIndex(lx, y, lz)] = BlockOakLog
		}
	}
	for dy := trunkH - 3; dy <= trunkH+1; dy++ {
		radius := 2
		if dy >= trunkH {
			radius = 1
		}
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				if abs(dx)+abs(dz) > radius+1 {
					continue
				}
				y := baseY + dy
				if y >= ChunkHeight {
					continue
				}
				nx := lx + dx
				nz := lz + dz
				if nx < 0 || nx >= ChunkSize || nz < 0 || nz >= ChunkSize {
					continue
				}
				idx := BlockIndex(nx, y, nz)
				if c.Blocks[idx] == BlockAir {
					c.Blocks[idx] = BlockLeaves
				}
			}
		}
	}
}

func carveSpawnPlaza(c *Chunk) {
	h := HeightAt(0, 0)
	for lz := 0; lz < 7; lz++ {
		for lx := 0; lx < 7; lx++ {
			maxY := h + 4
			if maxY > ChunkHeight {
				maxY = ChunkHeight
			}
			for y := h; y < maxY; y++ {
				c.Blocks[BlockIndex(lx, y, lz)] = BlockAir
			}
			c.Blocks[BlockIndex(lx, h, lz)] = BlockOakPlanks
			if lx == 0 || lx == 6 || lz == 0 || lz == 6 {
				if h+1 < ChunkHeight {
					c.Blocks[BlockIndex(lx, h+1, lz)] = BlockCobble
				}
			}
		}
	}
}

func smoothNoise(x, z int, scale float64) float64 {
	fx := float64(x) / scale
	fz := float64(z) / scale
	x0 := int(math.Floor(fx))
	z0 := int(math.Floor(fz))
	tx := fx - float64(x0)
	tz := fz - float64(z0)
	v00 := hash01(x0, z0)
	v10 := hash01(x0+1, z0)
	v01 := hash01(x0, z0+1)
	v11 := hash01(x0+1, z0+1)
	ix0 := lerp(v00, v10, fade(tx))
	ix1 := lerp(v01, v11, fade(tx))
	return lerp(ix0, ix1, fade(tz))*2.0 - 1.0
}

func hash01(x, z int) float64 {
	n := uint32(x)*73856093 ^ uint32(z)*19349663
	n ^= n << 13
	n ^= n >> 17
	n ^= n << 5
	return float64(n&0xffff) / 65535.0
}

func fade(t float64) float64 {
	return t * t * (3 - 2*t)
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
