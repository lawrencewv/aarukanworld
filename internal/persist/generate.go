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
)

// GenerateChunk fills a new column with a deterministic heightmap + spawn plaza.
func GenerateChunk(coord ChunkCoord) *Chunk {
	c := EmptyChunk(coord)
	ox := int(coord.X) * ChunkSize
	oz := int(coord.Z) * ChunkSize
	for lz := 0; lz < ChunkSize; lz++ {
		for lx := 0; lx < ChunkSize; lx++ {
			wx := ox + lx
			wz := oz + lz
			h := HeightAt(wx, wz)
			for y := 0; y < ChunkHeight; y++ {
				c.Blocks[BlockIndex(lx, y, lz)] = columnBlock(wx, y, wz, h)
			}
		}
	}
	// Include neighbor trunks so canopies aren't clipped at chunk borders.
	const treeMargin = 2
	for wz := oz - treeMargin; wz < oz+ChunkSize+treeMargin; wz++ {
		for wx := ox - treeMargin; wx < ox+ChunkSize+treeMargin; wx++ {
			h := HeightAt(wx, wz)
			if h+8 >= ChunkHeight || !shouldPlantTree(wx, wz) {
				continue
			}
			plantTreeInChunk(c, ox, oz, wx, h+1, wz)
		}
	}
	if coord.X == 0 && coord.Z == 0 {
		carveSpawnPlaza(c)
	}
	return c
}

// HeightAt is the surface Y for world (x, z).
// Rolling hills with frequent mountain peaks (mirrors aarukanclient VoxelWorld.height_at).
func HeightAt(x, z int) int {
	n := smoothNoise(x, z, 64.0)
	d := smoothNoise(x+19, z-7, 28.0)
	mRaw := smoothNoise(x-41, z+23, 140.0)
	m := mRaw + 0.2
	if m < 0 {
		m = 0
	}
	m = m * m
	h := int(math.Round(26.0 + n*5.0 + d*1.5 + m*28.0))
	if h < 4 {
		h = 4
	}
	if h > ChunkHeight-8 {
		h = ChunkHeight - 8
	}
	return h
}

func columnBlock(x, y, z, surface int) uint16 {
	if y > surface {
		return BlockAir
	}
	if y == surface {
		if surface <= 18 {
			return BlockSand
		}
		return BlockGrass
	}
	if y >= surface-3 {
		return BlockDirt
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
