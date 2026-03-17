package classifier

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestRules(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")

	yaml := `
source_category_boost: 2.0
categories:
  ai_ml:
    name: "AI/ML"
    keywords:
      - GPT
      - ChatGPT
      - large language model
      - LLM
      - 人工智能
      - 机器学习
      - deep learning
      - neural network
      - OpenAI
    boost_keywords:
      - LLM
      - GPT
      - ChatGPT
  tech_frontier:
    name: "科技前沿"
    keywords:
      - quantum computing
      - blockchain
      - 量子计算
      - 芯片
      - semiconductor
      - 6G
    boost_keywords:
      - 量子计算
      - semiconductor
  business:
    name: "商业动态"
    keywords:
      - funding
      - 融资
      - IPO
      - acquisition
      - 并购
      - unicorn
      - 独角兽
    boost_keywords:
      - IPO
      - 融资
  open_source:
    name: "开源生态"
    keywords:
      - open source
      - 开源
      - GitHub
      - Kubernetes
      - PyTorch
    boost_keywords:
      - open source
      - 开源
  research:
    name: "学术研究"
    keywords:
      - arXiv
      - paper
      - 论文
      - NeurIPS
      - breakthrough
      - state-of-the-art
    boost_keywords:
      - arXiv
      - NeurIPS
      - 论文
  policy:
    name: "政策监管"
    keywords:
      - regulation
      - 监管
      - EU AI Act
      - GDPR
      - 隐私
      - lawsuit
      - 诉讼
    boost_keywords:
      - EU AI Act
      - 监管
  product:
    name: "产品发布"
    keywords:
      - launch
      - release
      - 发布
      - announce
      - iPhone
      - Pixel
      - version
      - update
    boost_keywords:
      - launch
      - 发布
      - iPhone
  general:
    name: "综合资讯"
    keywords: []
    boost_keywords: []
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("write test rules: %v", err)
	}
	return path
}

func TestNewKeywordClassifier(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}
	if kc == nil {
		t.Fatal("expected non-nil classifier")
	}

	cats := kc.Categories()
	if len(cats) != 8 {
		t.Errorf("expected 8 categories, got %d", len(cats))
	}
}

func TestClassify_AI(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	tests := []struct {
		name    string
		title   string
		summary string
		want    string
	}{
		{
			name:    "GPT in title",
			title:   "OpenAI Announces GPT-5 with Improved Reasoning",
			summary: "The new model shows significant improvements in math and coding.",
			want:    "AI/ML",
		},
		{
			name:    "ChatGPT in summary",
			title:   "New Features Coming to AI Assistant",
			summary: "ChatGPT now supports multimodal input including images and audio.",
			want:    "AI/ML",
		},
		{
			name:    "LLM in title",
			title:   "Building Better LLMs for Enterprise",
			summary: "Companies are investing heavily in large language models.",
			want:    "AI/ML",
		},
		{
			name:    "Chinese AI title",
			title:   "人工智能大模型突破：新架构提升推理能力",
			summary: "国内研究团队提出新型深度学习架构，在多项基准测试中取得领先。",
			want:    "AI/ML",
		},
		{
			name:    "deep learning",
			title:   "Deep Learning Advances in Computer Vision",
			summary: "New neural network architectures achieve state-of-the-art on ImageNet.",
			want:    "AI/ML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kc.ClassifySimple(tt.title, tt.summary)
			if got != tt.want {
				t.Errorf("ClassifySimple(%q, %q) = %q, want %q", tt.title, tt.summary, got, tt.want)
			}
		})
	}
}

func TestClassify_Business(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	tests := []struct {
		name    string
		title   string
		summary string
		want    string
	}{
		{
			name:    "funding round",
			title:   "AI Startup Raises $100M in Series B Funding",
			summary: "The round was led by Sequoia Capital.",
			want:    "商业动态",
		},
		{
			name:    "Chinese funding",
			title:   "国内AI独角兽完成新一轮融资",
			summary: "估值超过50亿美元，本轮融资将用于扩大研发团队。",
			want:    "商业动态",
		},
		{
			name:    "IPO",
			title:   "Tech Company Files for IPO",
			summary: "The AI chip maker plans to list on NASDAQ.",
			want:    "商业动态",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kc.ClassifySimple(tt.title, tt.summary)
			if got != tt.want {
				t.Errorf("ClassifySimple(%q, %q) = %q, want %q", tt.title, tt.summary, got, tt.want)
			}
		})
	}
}

func TestClassify_OpenSource(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	got := kc.ClassifySimple(
		"New Open Source Framework for ML Training",
		"The GitHub repository has gained 10k stars in just one week.",
	)
	if got != "开源生态" {
		t.Errorf("expected 开源生态, got %q", got)
	}
}

func TestClassify_Research(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	got := kc.ClassifySimple(
		"New Paper on arXiv Proposes Novel Attention Mechanism",
		"The research will be presented at NeurIPS 2026.",
	)
	if got != "学术研究" {
		t.Errorf("expected 学术研究, got %q", got)
	}
}

func TestClassify_Policy(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	tests := []struct {
		name    string
		title   string
		summary string
		want    string
	}{
		{
			name:    "EU AI Act",
			title:   "EU AI Act Comes into Effect: What Companies Need to Know",
			summary: "The new regulation imposes strict requirements on high-risk AI systems.",
			want:    "政策监管",
		},
		{
			name:    "Chinese regulation",
			title:   "AI监管新规出台，算法推荐服务须备案",
			summary: "国家网信办发布新规，要求所有算法推荐服务进行安全评估。",
			want:    "政策监管",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kc.ClassifySimple(tt.title, tt.summary)
			if got != tt.want {
				t.Errorf("ClassifySimple(%q, %q) = %q, want %q", tt.title, tt.summary, got, tt.want)
			}
		})
	}
}

func TestClassify_Product(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	got := kc.ClassifySimple(
		"Apple to Launch New iPhone with AI Features",
		"The upcoming release will include an updated version of Siri.",
	)
	if got != "产品发布" {
		t.Errorf("expected 产品发布, got %q", got)
	}
}

func TestClassify_TechFrontier(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	tests := []struct {
		name    string
		title   string
		summary string
		want    string
	}{
		{
			name:    "quantum",
			title:   "Google Achieves Quantum Computing Breakthrough",
			summary: "The new quantum processor can solve problems that classical computers cannot.",
			want:    "科技前沿",
		},
		{
			name:    "chip",
			title:   "TSMC Announces 2nm Semiconductor Process",
			summary: "The new chip manufacturing technology promises significant power savings.",
			want:    "科技前沿",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kc.ClassifySimple(tt.title, tt.summary)
			if got != tt.want {
				t.Errorf("ClassifySimple(%q, %q) = %q, want %q", tt.title, tt.summary, got, tt.want)
			}
		})
	}
}

func TestClassify_Fallback(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	// Article with no matching keywords should fall back to 综合资讯
	got := kc.ClassifySimple(
		"Random News Article About Weather",
		"The weather today is sunny with mild temperatures expected throughout the week.",
	)
	if got != "综合资讯" {
		t.Errorf("expected 综合资讯 (fallback), got %q", got)
	}
}

func TestClassify_EmptyInput(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	got := kc.ClassifySimple("", "")
	if got != "综合资讯" {
		t.Errorf("expected 综合资讯 for empty input, got %q", got)
	}
}

func TestSourceCategoryBoost(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	// Article with ambiguous title but source category "AI/ML" should get AI boost
	got := kc.ClassifyWithSource(
		"New System Improves Performance",
		"The system achieves better results on standard benchmarks.",
		[]string{"AI/ML"},
	)
	// With source boost, AI/ML should have score 2.0 while others have 0
	// So it should still classify as AI/ML even without keyword match
	if got != "AI/ML" {
		t.Errorf("expected AI/ML with source category boost, got %q", got)
	}
}

func TestSourceCategoryBoost_Weighted(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	// "GitHub" is an open_source keyword (not boost) → score 1.0 * 1.5 = 1.5 (in title)
	// AI/ML has source category boost 2.0 (no keyword match)
	// AI/ML should win because 2.0 > 1.5
	got := kc.ClassifyWithSource(
		"GitHub Project Gains Traction",
		"A new project is now available for download.",
		[]string{"AI/ML"},
	)
	if got != "AI/ML" {
		t.Errorf("expected AI/ML to win over open_source due to source boost, got %q", got)
	}
}

func TestClassify_Scores(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	result := kc.Classify(&ArticleInput{
		Title:   "OpenAI Releases GPT-5 with ChatGPT Integration",
		Summary: "The new LLM achieves breakthrough performance.",
	})

	if result.Category != "AI/ML" {
		t.Errorf("expected AI/ML, got %q", result.Category)
	}

	if result.Scores == nil {
		t.Fatal("expected non-nil scores")
	}

	// AI/ML should have the highest score
	aiScore := result.Scores["ai_ml"]
	for key, score := range result.Scores {
		if key != "ai_ml" && score >= aiScore {
			t.Errorf("category %s (score %.2f) should not >= ai_ml (score %.2f)", key, score, aiScore)
		}
	}
}

func TestReload(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	// Before reload, should classify based on original rules
	got1 := kc.ClassifySimple("GPT-5 Released", "OpenAI announces new model")
	if got1 != "AI/ML" {
		t.Errorf("before reload: expected AI/ML, got %q", got1)
	}

	// Modify rules to remove AI keywords
	yaml := `
source_category_boost: 2.0
categories:
  ai_ml:
    name: "AI/ML"
    keywords: []
    boost_keywords: []
  tech_frontier:
    name: "科技前沿"
    keywords:
      - GPT
      - OpenAI
    boost_keywords:
      - GPT
  business:
    name: "商业动态"
    keywords: []
    boost_keywords: []
  open_source:
    name: "开源生态"
    keywords: []
    boost_keywords: []
  research:
    name: "学术研究"
    keywords: []
    boost_keywords: []
  policy:
    name: "政策监管"
    keywords: []
    boost_keywords: []
  product:
    name: "产品发布"
    keywords: []
    boost_keywords: []
  general:
    name: "综合资讯"
    keywords: []
    boost_keywords: []
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("write modified rules: %v", err)
	}

	// Reload
	if err := kc.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// After reload, GPT should match tech_frontier instead of ai_ml
	got2 := kc.ClassifySimple("GPT-5 Released", "OpenAI announces new model")
	if got2 != "科技前沿" {
		t.Errorf("after reload: expected 科技前沿, got %q", got2)
	}
}

func TestCaseInsensitive(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	// Test various case combinations
	got := kc.ClassifySimple(
		"gpt and chatgpt advances in deep LEARNING",
		"New NEURAL NETWORK architectures are being developed.",
	)
	if got != "AI/ML" {
		t.Errorf("expected case-insensitive match for AI/ML, got %q", got)
	}
}

func TestTitleVsSummaryWeight(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	// "paper" appears in title (research keyword) and "GitHub" appears in summary (open_source keyword, NOT boost)
	// title weight 1.5x for paper → research: 1.0 * 1.5 = 1.5
	// summary weight 1.0x for GitHub → open_source: 1.0 * 1.0 = 1.0
	// research should win due to title weight
	got := kc.ClassifySimple(
		"New Paper Proposes Novel Architecture",
		"The implementation is available on GitHub for developers.",
	)
	if got != "学术研究" {
		t.Errorf("expected 学术研究 (title keyword should weigh more), got %q", got)
	}
}

func TestChineseKeywords(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	tests := []struct {
		title   string
		summary string
		want    string
	}{
		{"人工智能新突破", "国内研究团队在机器学习领域取得重大进展", "AI/ML"},
		{"量子计算实现里程碑", "新型量子处理器实现100量子比特", "科技前沿"},
		{"AI初创公司完成融资", "本轮融资由红杉资本领投", "商业动态"},
		{"开源大模型发布", "GitHub上获得广泛关注", "开源生态"},
	}

	for _, tt := range tests {
		got := kc.ClassifySimple(tt.title, tt.summary)
		if got != tt.want {
			t.Errorf("ClassifySimple(%q, %q) = %q, want %q", tt.title, tt.summary, got, tt.want)
		}
	}
}

func TestDebugClassify(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	debug := kc.DebugClassify(&ArticleInput{
		Title:   "OpenAI Releases GPT-5",
		Summary: "The new model shows breakthrough performance.",
	})

	if !strings.Contains(debug, "AI/ML") {
		t.Errorf("debug output should contain AI/ML: %s", debug)
	}
	if !strings.Contains(debug, "Scores:") {
		t.Errorf("debug output should contain scores: %s", debug)
	}
}

func TestMultipleSourceCategories(t *testing.T) {
	path := createTestRules(t)
	kc, err := NewKeywordClassifier(path)
	if err != nil {
		t.Fatalf("NewKeywordClassifier: %v", err)
	}

	// Multiple source categories should boost multiple categories
	got := kc.ClassifyWithSource(
		"New Research Paper Published",
		"Published on arXiv, the paper discusses open source approaches.",
		[]string{"学术研究", "开源生态"},
	)
	// Both research and open_source get +2.0 boost
	// research: boost 2.0 + arXiv keyword in summary 1.0 = 3.0
	// open_source: boost 2.0 + open source keyword in summary 1.0 = 3.0
	// Tie: first one wins (research comes before open_source in KnownCategories)
	if got != "学术研究" {
		t.Errorf("expected 学术研究 with multiple source boosts, got %q", got)
	}
}
