// Package services — FSRS-6 (Free Spaced Repetition Scheduler v6) 核心公式。
// 内嵌 FSRS-6 的 21 参数遗忘曲线和稳定性计算，不引入 go-fsrs 外部依赖。
// 参考: github.com/open-spaced-repetition/fsrs4anki/wiki/The-Algorithm
// 参考: borretti.me/article/implementing-fsrs-in-100-lines
package services

import "math"

// FSRSGrade 表示记忆回忆的评分等级 (1-4)。
type FSRSGrade int

const (
	FSRSAgain FSRSGrade = 1 // 完全遗忘
	FSRSHard  FSRSGrade = 2 // 困难回忆
	FSRSGood  FSRSGrade = 3 // 正常回忆
	FSRSEasy  FSRSGrade = 4 // 轻松回忆
)

// FSRSParams 持有 FSRS-6 的 21 个可训练参数 (w0-w20)。
// 支持 per-user 个性化：每个用户可拥有独立的参数集。
type FSRSParams struct {
	W [21]float64
}

// DefaultFSRSParams 返回 FSRS-6 官方默认参数。
// 来源: github.com/open-spaced-repetition/fsrs4anki/wiki/The-Algorithm
var DefaultFSRSParams = FSRSParams{
	W: [21]float64{
		0.212, 1.2931, 2.3065, 8.2956, // w0-w3: 初始稳定性 S0(G)
		6.4133,                // w4: 初始难度 D0 基准
		0.8334,                // w5: 难度指数衰减
		3.0194,                // w6: 难度增量系数
		0.001,                 // w7: 难度均值回归权重
		1.8722, 0.1666, 0.796, // w8-w10: 回忆后稳定性增长
		1.4835, 0.0614, 0.2629, 1.6483, // w11-w14: 遗忘后稳定性
		0.6014, 1.8729, // w15-w16: Hard/Easy 修正
		0.5425, 0.0912, 0.0658, // w17-w19: 同日复习
		0.1542, // w20: 幂律遗忘曲线指数
	},
}

// Retrievability 计算幂律遗忘曲线上的检索概率。
// FSRS-6 公式: R = (1 + factor × t / S)^decay
// 其中 decay = -w20, factor = 0.9^(1/decay) - 1，确保 R(S, S) = 0.9。
//
// 参数:
//   - elapsedDays: 自上次复习以来经过的天数 (t)
//   - stability: 当前稳定性 (S)，单位为天
//   - w20: 幂律遗忘曲线指数（正值）
//
// 返回值: [0, 1] 范围的检索概率
func Retrievability(elapsedDays, stability, w20 float64) float64 {
	if stability <= 0 {
		return 0
	}
	if elapsedDays <= 0 {
		return 1.0
	}
	if w20 <= 0 {
		w20 = DefaultFSRSParams.W[20]
	}
	decay := -w20
	factor := math.Pow(0.9, 1.0/decay) - 1
	return math.Pow(1+factor*elapsedDays/stability, decay)
}

// InitialStability 返回首次学习后的初始稳定性 S0(G)。
// FSRS-6 公式: S0(G) = w[G-1]
func InitialStability(p *FSRSParams, grade FSRSGrade) float64 {
	if grade < FSRSAgain || grade > FSRSEasy {
		grade = FSRSGood
	}
	s := p.W[grade-1]
	// 下限保护：稳定性不低于 0.01 天
	if s < 0.01 {
		s = 0.01
	}
	return s
}

// InitialDifficulty 返回首次学习后的初始难度 D0(G)。
// FSRS-6 公式: D0(G) = w4 - exp(w5 × (G - 1)) + 1
// 结果钳位到 [1, 10]。
func InitialDifficulty(p *FSRSParams, grade FSRSGrade) float64 {
	d := p.W[4] - math.Exp(p.W[5]*float64(grade-1)) + 1
	return clampDifficulty(d)
}

// NextDifficulty 计算一次复习后的新难度（含均值回归）。
// FSRS-6 公式:
//
//	D' = D - w6 × (G - 3)
//	D'' = w7 × D0(4) + (1 - w7) × D'
//
// 其中 D0(4) 是 Easy 等级的初始难度，用于均值回归。
func NextDifficulty(p *FSRSParams, d float64, grade FSRSGrade) float64 {
	dPrime := d - p.W[6]*(float64(grade)-3)
	d0Easy := InitialDifficulty(p, FSRSEasy)
	dDoublePrime := p.W[7]*d0Easy + (1-p.W[7])*dPrime
	return clampDifficulty(dDoublePrime)
}

// StabilityAfterRecall 计算成功回忆后的新稳定性。
// FSRS-6 公式:
//
//	S'r = S × (1 + exp(w8) × (11 - D) × S^(-w9) × (exp(w10×(1-R)) - 1) × h × b)
//
// 其中:
//   - h = Hard 修正因子 (grade=2 时 = w15, 否则 = 1)
//   - b = Easy 修正因子 (grade=4 时 = w16, 否则 = 1)
func StabilityAfterRecall(p *FSRSParams, d, s, r float64, grade FSRSGrade) float64 {
	h := 1.0
	b := 1.0
	if grade == FSRSHard {
		h = p.W[15]
	}
	if grade == FSRSEasy {
		b = p.W[16]
	}

	newS := s * (1 + math.Exp(p.W[8])*(11-d)*math.Pow(s, -p.W[9])*(math.Exp(p.W[10]*(1-r))-1)*h*b)

	// 新稳定性不能低于原稳定性的下界
	if newS < 0.01 {
		newS = 0.01
	}
	return newS
}

// StabilityAfterLapse 计算遗忘（Lapse）后的新稳定性。
// FSRS-6 公式:
//
//	S'f = w11 × D^(-w12) × ((S+1)^w13 - 1) × exp(w14 × (1 - R))
//
// 结果下限为 0.01 天。
func StabilityAfterLapse(p *FSRSParams, d, s, r float64) float64 {
	newS := p.W[11] * math.Pow(d, -p.W[12]) * (math.Pow(s+1, p.W[13]) - 1) * math.Exp(p.W[14]*(1-r))
	if newS < 0.01 {
		newS = 0.01
	}
	return newS
}

// SameDayStability 计算同日复习（间隔 < 1 天）后的稳定性。
// FSRS-6 公式:
//
//	S' = S × exp(w17 × (G - 3 + w18)) × S^(-w19)
//
// 注：同日复习仅在 S' > S 时取新值，否则保持不变。
func SameDayStability(p *FSRSParams, s float64, grade FSRSGrade) float64 {
	newS := s * math.Exp(p.W[17]*(float64(grade)-3+p.W[18])) * math.Pow(s, -p.W[19])
	if newS < s {
		return s
	}
	if newS < 0.01 {
		return 0.01
	}
	return newS
}

// clampDifficulty 将难度值钳位到 [1, 10] 范围。
func clampDifficulty(d float64) float64 {
	if d < 1 {
		return 1
	}
	if d > 10 {
		return 10
	}
	return d
}
