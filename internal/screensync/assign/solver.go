package assign

import "lightsync/internal/lights"

// hungarianSolve finds the minimum-cost assignment in an nRows × nCols cost
// matrix using the Jonker-Volgenant shortest-augmenting-path variant (O(n³)).
// Returns assignment[rowIdx] = colIdx.
func hungarianSolve(cost [][]float64, nRows, nCols int) []int {
	const inf = 1e18
	n := nRows
	if nCols > n {
		n = nCols
	}

	// Pad to square with large costs for dummy entries.
	c := make([][]float64, n)
	for i := range c {
		c[i] = make([]float64, n)
		for j := range c[i] {
			if i < nRows && j < nCols {
				c[i][j] = cost[i][j]
			} else {
				c[i][j] = inf
			}
		}
	}

	u := make([]float64, n+1)
	v := make([]float64, n+1)
	p := make([]int, n+1) // p[j] = row assigned to column j (1-indexed)
	way := make([]int, n+1)

	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		minV := make([]float64, n+1)
		used := make([]bool, n+1)
		for j := range minV {
			minV[j] = inf
		}
		for {
			used[j0] = true
			i0 := p[j0]
			delta := inf
			j1 := -1
			for j := 1; j <= n; j++ {
				if !used[j] {
					cur := c[i0-1][j-1] - u[i0] - v[j]
					if cur < minV[j] {
						minV[j] = cur
						way[j] = j0
					}
					if minV[j] < delta {
						delta = minV[j]
						j1 = j
					}
				}
			}
			if j1 < 0 {
				break
			}
			for j := 0; j <= n; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minV[j] -= delta
				}
			}
			j0 = j1
			if p[j0] == 0 {
				break
			}
		}
		for j0 != 0 {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
		}
	}

	ans := make([]int, n)
	for j := 1; j <= n; j++ {
		if p[j] > 0 {
			ans[p[j]-1] = j - 1
		}
	}
	if nRows < n {
		return ans[:nRows]
	}
	return ans
}

// applyAssignment converts an assignment slice (assignment[colorIdx] = deviceIdx)
// into a device→color map. Devices without an assignment fall back to colors[0].
func applyAssignment(assignment []int, colors []lights.Color, deviceIDs []string) map[string]lights.Color {
	result := make(map[string]lights.Color, len(deviceIDs))
	assigned := make(map[int]bool)
	for colorIdx, devIdx := range assignment {
		if colorIdx < len(colors) && devIdx < len(deviceIDs) {
			result[deviceIDs[devIdx]] = colors[colorIdx]
			assigned[devIdx] = true
		}
	}
	for i, id := range deviceIDs {
		if _, ok := result[id]; !ok && !assigned[i] {
			result[id] = colors[0]
		}
	}
	return result
}
