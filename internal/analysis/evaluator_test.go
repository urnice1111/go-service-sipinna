package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Servidor falso que imita la API de Jev y revisa la petición que le llega.
func fakeJev(t *testing.T, status int, answers map[string]Answer) (*httptest.Server, *jevRequest) {
	t.Helper()
	var got jevRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer clave-prueba" {
			t.Errorf("Authorization incorrecto: %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("petición inválida: %v", err)
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-test", "answers": answers})
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func clientFor(srv *httptest.Server) *JevClient {
	c := NewJevClient("clave-prueba")
	c.URL = srv.URL
	return c
}

var reporteReal = ReportInput{
	Description:      "Niño de unos 8 años vendiendo chicles entre los autos en el semáforo, solo, a las 10 de la noche.\nDirección: Av. Siempre Viva 123\nCondición: Solo / Sola",
	WorkType:         "Semáforos",
	ChildrenAge:      "8 - 10 años",
	ChildrenQuantity: 1,
	PhotoCount:       1,
}

func TestReporteLegitimo(t *testing.T) {
	srv, req := fakeJev(t, 200, map[string]Answer{
		"coherencia":     {Type: "score", Score: 0.2},
		"no_relacionado": {Type: "noul", Noul: 0.02},
		"inconsistente":  {Type: "noul", Noul: 0.05},
	})

	res, err := (&Evaluator{Jev: clientFor(srv)}).Evaluate(context.Background(), reporteReal)
	if err != nil {
		t.Fatal(err)
	}
	if !res.UsedLLM || res.Score > 0.1 {
		t.Fatalf("un reporte creíble debería tener sospecha baja, obtuvo %+v", res)
	}
	desc, _ := (*req).State.(map[string]any)["descripcion"].(string)
	if strings.Contains(desc, "Siempre Viva") {
		t.Fatal("la dirección no debe enviarse a Jev")
	}
	if len(req.Questions) != 3 || req.Model != "jev-latest" {
		t.Fatalf("petición inesperada: %+v", req)
	}
	if req.Questions["coherencia"].Type != "score" || req.Questions["no_relacionado"].Type != "noul" {
		t.Fatal("tipos de pregunta incorrectos")
	}
}

func TestReporteBroma(t *testing.T) {
	srv, _ := fakeJev(t, 200, map[string]Answer{
		"coherencia":     {Type: "score", Score: 2.9},
		"no_relacionado": {Type: "noul", Noul: 0.95},
		"inconsistente":  {Type: "noul", Noul: 0.7},
	})
	in := ReportInput{Description: "jajaja prueba", Anonymous: true}

	res, err := (&Evaluator{Jev: clientFor(srv)}).Evaluate(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Score < 0.8 {
		t.Fatalf("una broma debería tener sospecha alta, obtuvo %.2f", res.Score)
	}
	if len(res.Reasons) < 3 {
		t.Fatalf("faltan motivos: %v", res.Reasons)
	}
}

func TestSoloReglas(t *testing.T) {
	in := reporteReal
	in.SameUserNearby24h = 1
	in.SameUserLastHour = 3

	res, err := (&Evaluator{}).Evaluate(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if res.UsedLLM || res.Score != 0.45 {
		t.Fatalf("esperaba 0.45 solo con reglas, obtuvo %+v", res)
	}
}

func TestErroresDeJev(t *testing.T) {
	saturado, _ := fakeJev(t, 529, nil)
	res, err := (&Evaluator{Jev: clientFor(saturado)}).Evaluate(context.Background(), reporteReal)
	if err == nil || !IsRetryable(err) {
		t.Fatalf("un 529 debe ser reintentable: %v", err)
	}
	if res.UsedLLM {
		t.Fatal("si Jev falla, el resultado debe ser solo de reglas")
	}

	if IsAccountError(err) {
		t.Fatal("un 529 no es un problema de la cuenta")
	}

	for _, status := range []int{401, 402, 403} {
		cuenta, _ := fakeJev(t, status, nil)
		_, err = (&Evaluator{Jev: clientFor(cuenta)}).Evaluate(context.Background(), reporteReal)
		if err == nil || IsRetryable(err) || !IsAccountError(err) {
			t.Fatalf("un %d debe detener el análisis sin reintentar: %v", status, err)
		}
	}

	rechazado, _ := fakeJev(t, 422, nil)
	_, err = (&Evaluator{Jev: clientFor(rechazado)}).Evaluate(context.Background(), reporteReal)
	if err == nil || IsRetryable(err) || IsAccountError(err) {
		t.Fatalf("un 422 es un problema del reporte, no de la cuenta: %v", err)
	}
}

func TestSinClaveNoHayCliente(t *testing.T) {
	if NewJevClient("") != nil {
		t.Fatal("sin API key no debe crearse cliente")
	}
}
