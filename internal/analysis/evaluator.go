package analysis

import (
	"context"
	"math"
	"strings"
	"unicode"
)

// ReportInput son los datos de un reporte que se analizan.
// No incluye datos del ciudadano (nombre, correo, teléfono) ni fotos:
// a Jev solo se le manda lo necesario para evaluar el relato.
type ReportInput struct {
	Description      string
	WorkType         string
	ChildrenAge      string
	ChildrenQuantity int
	SightingTime     string

	// Señales calculadas en la base de datos
	PhotoCount        int
	Anonymous         bool // sin ciudadano_id
	SameUserNearby24h int  // otros reportes del mismo usuario a <~200 m en 24 h
	SameUserLastHour  int  // otros reportes del mismo usuario en la hora previa
}

// Result es la evaluación final.
type Result struct {
	Score   float64  // 0 = parece legítimo, 1 = muy probablemente falso
	UsedLLM bool     // true si Jev participó en el puntaje
	Reasons []string // motivos que subieron el puntaje, para mostrar al personal
}

// Asker es lo que el evaluador necesita de Jev (permite simularlo en pruebas).
type Asker interface {
	Ask(ctx context.Context, state any, questions map[string]Question) (map[string]Answer, error)
}

// Evaluator calcula el puntaje de sospecha de un reporte.
type Evaluator struct {
	Jev Asker // nil = solo reglas
}

// Preguntas para Jev. Se definen una vez y se reutilizan en cada reporte.
var reportQuestions = map[string]Question{
	"coherencia": Score(
		"¿Qué tan creíble y concreto es este reporte de un posible caso de trabajo infantil en México?",
		"Relato concreto y creíble: describe qué hace el menor, dónde o cuándo, con detalles observables",
		"Relato creíble pero breve o general, con pocos detalles",
		"Relato vago, contradictorio o difícil de creer",
		"No es un reporte real: broma, prueba, publicidad o spam, texto sin sentido, insultos o un tema sin relación",
	),
	"no_relacionado": Noul(
		"¿El reporte es una broma, una prueba, spam, publicidad, insultos o trata de algo que no es trabajo infantil?",
		"Es broma, prueba, spam o no tiene relación con trabajo infantil",
		"Es un intento genuino de reportar a un menor trabajando",
	),
	"inconsistente": Noul(
		"¿La descripción contradice la categoría de trabajo, la edad aproximada o la cantidad de menores indicadas?",
		"Hay una contradicción clara entre la descripción y los datos seleccionados",
		"Los datos seleccionados son compatibles con la descripción",
	),
}

// Evaluate calcula el puntaje. Si Jev falla, regresa el error para que el llamador
// decida si reintenta más tarde o se queda con el puntaje de reglas (res siempre es válido).
func (e *Evaluator) Evaluate(ctx context.Context, in ReportInput) (Result, error) {
	rules, reasons := ruleScore(in)
	res := Result{Score: round2(rules), Reasons: reasons}

	if e.Jev == nil {
		return res, nil
	}

	answers, err := e.Jev.Ask(ctx, jevState(in), reportQuestions)
	if err != nil {
		return res, err
	}

	llm, llmReasons := llmScore(answers)

	// Se combinan como probabilidades independientes ("OR ruidoso"):
	// el reporte es sospechoso si lo indica el análisis del texto o las reglas.
	res.Score = round2(1 - (1-llm)*(1-rules))
	res.UsedLLM = true
	res.Reasons = append(llmReasons, reasons...)
	return res, nil
}

// jevState arma lo que ve Jev. Se quita la dirección escrita (la agrega la app
// al final de la descripción) porque no ayuda a evaluar el relato y es un dato sensible.
func jevState(in ReportInput) map[string]any {
	return map[string]any{
		"descripcion":           stripAddress(in.Description),
		"categorias_de_trabajo": in.WorkType,
		"edad_aproximada":       in.ChildrenAge,
		"cantidad_de_menores":   in.ChildrenQuantity,
		"hora_del_avistamiento": in.SightingTime,
		"adjunto_fotos":         in.PhotoCount > 0,
	}
}

// llmScore convierte las respuestas de Jev en un número de 0 a 1.
func llmScore(a map[string]Answer) (float64, []string) {
	coherencia := clamp01(a["coherencia"].Score / 3) // niveles 0..3
	noRelacionado := clamp01(a["no_relacionado"].Noul)
	inconsistente := clamp01(a["inconsistente"].Noul)

	score := 0.5*coherencia + 0.3*noRelacionado + 0.2*inconsistente

	var reasons []string
	if noRelacionado >= 0.5 {
		reasons = append(reasons, "El texto parece broma, prueba o no relacionado con trabajo infantil")
	}
	if coherencia >= 0.5 {
		reasons = append(reasons, "El relato es vago o poco creíble")
	}
	if inconsistente >= 0.5 {
		reasons = append(reasons, "La descripción no coincide con la categoría, edad o cantidad indicadas")
	}
	return clamp01(score), reasons
}

// ruleScore aplica reglas simples que no necesitan IA.
// Cada regla suma puntos; el total se limita a 1.
func ruleScore(in ReportInput) (float64, []string) {
	score := 0.0
	var reasons []string

	if meaningfulLength(stripAddress(in.Description)) < 25 {
		score += 0.15
		reasons = append(reasons, "Descripción muy corta")
	}
	if in.SameUserNearby24h > 0 {
		score += 0.25
		reasons = append(reasons, "El mismo usuario ya reportó en ese lugar en las últimas 24 horas")
	}
	if in.SameUserLastHour >= 3 {
		score += 0.20
		reasons = append(reasons, "Muchos reportes del mismo usuario en una hora")
	}
	if in.PhotoCount == 0 && in.Anonymous {
		score += 0.05
		reasons = append(reasons, "Reporte anónimo y sin fotos")
	}

	return clamp01(score), reasons
}

// stripAddress quita la línea "Dirección: ..." que la app agrega a la descripción.
func stripAddress(desc string) string {
	lines := strings.Split(desc, "\n")
	kept := lines[:0]
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "Dirección:") {
			continue
		}
		kept = append(kept, l)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// meaningfulLength cuenta letras y números (ignora espacios, signos y la línea de condición).
func meaningfulLength(desc string) int {
	n := 0
	for _, l := range strings.Split(desc, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "Condición:") {
			continue
		}
		for _, r := range l {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				n++
			}
		}
	}
	return n
}

// clamp01 limita v al rango [0, 1]; NaN se convierte en 0.
func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return math.Max(0, math.Min(1, v))
}

// round2 redondea v a dos decimales.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
