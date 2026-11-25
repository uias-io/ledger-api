package zlog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"ledger/src/cfgtypes"
	"log/slog"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"

	"github.com/natefinch/lumberjack"
)

// ---------------- context key ----------------
type loggerCtxKey struct{}
type requestIdCtxKey struct{}

type ctxValue struct {
	logger    *slog.Logger
	requestID string
}

type Logger struct {
	ctx  context.Context
	slog *slog.Logger
}

// FromCtx 初始化入口
func FromCtx(ctx context.Context) *Logger {
	if ctx == nil {
		ctx = context.Background()
	}
	var rid string
	sl := slog.Default()
	if v, ok := ctx.Value(loggerCtxKey{}).(*ctxValue); ok && v != nil && v.logger != nil {
		sl = v.logger
		rid = v.requestID
	}
	if rid == "" {
		rid = RequestIDFromCtx(ctx)
	}
	if rid != "-" && rid != "" {
		sl = sl.With("request_id", rid)
	}
	return &Logger{ctx: ctx, slog: sl}
}

// NewCtxWithReqID 给 ctx 设置 request_id
func NewCtxWithReqID(ctx context.Context, requestId string) context.Context {
	return context.WithValue(ctx, requestIdCtxKey{}, requestId)
}

// RequestIDFromCtx 获取 request_id
func RequestIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return "-"
	}
	rid, ok := ctx.Value(requestIdCtxKey{}).(string)
	if ok && rid != "" {
		return rid
	}
	return "-"
}

// NewCtxWithLogger 注入 logger 和 requestID 到 ctx（老逻辑兼容，逐步弃用）
func NewCtxWithLogger(ctx context.Context, l *slog.Logger, requestId string) context.Context {
	if l == nil {
		l = slog.Default()
	}
	return context.WithValue(ctx, loggerCtxKey{}, &ctxValue{
		logger:    l,
		requestID: requestId,
	})
}

// ---------------- Logger 方法 ----------------
func (l *Logger) With(args ...any) *Logger {
	return &Logger{ctx: l.ctx, slog: l.slog.With(args...)}
}

func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{ctx: l.ctx, slog: l.slog.WithGroup(name)}
}

func (l *Logger) Slog() *slog.Logger {
	return l.slog
}

func (l *Logger) Ctx() context.Context {
	return l.ctx
}

// getCallerShort 在API入口获取调用方文件行号
// skip: 0 本函数，1调用本函数的方法，以此类推
func getCallerShort(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "?:?"
	}
	return fmt.Sprintf("%s:%d", path.Base(file), line)
}

func (l *Logger) Debug(args ...any) {
	if !l.slog.Enabled(l.ctx, slog.LevelDebug) {
		return
	}
	caller := getCallerShort(2) // skip=2：Debug -> getCallerShort，往上两层是业务调用处
	msg, kv := normalizeArgs(args)
	kv = append(kv, "_caller", caller)
	l.slog.Log(l.ctx, slog.LevelDebug, msg, kv...)
}

func (l *Logger) Info(args ...any) {
	if !l.slog.Enabled(l.ctx, slog.LevelInfo) {
		return
	}
	caller := getCallerShort(2)
	msg, kv := normalizeArgs(args)
	kv = append(kv, "_caller", caller)
	l.slog.Log(l.ctx, slog.LevelInfo, msg, kv...)
}

func (l *Logger) Warn(args ...any) {
	if !l.slog.Enabled(l.ctx, slog.LevelWarn) {
		return
	}
	caller := getCallerShort(2)
	msg, kv := normalizeArgs(args)
	kv = append(kv, "_caller", caller)
	l.slog.Log(l.ctx, slog.LevelWarn, msg, kv...)
}

func (l *Logger) Error(args ...any) {
	if !l.slog.Enabled(l.ctx, slog.LevelError) {
		return
	}
	caller := getCallerShort(2)
	msg, kv := normalizeArgs(args)
	kv = append(kv, "_caller", caller)
	l.slog.Log(l.ctx, slog.LevelError, msg, kv...)
}

// logAny 旧底层函数不再使用，保留签名便于编译，内部空实现
func (l *Logger) logAny(_ slog.Level, _ ...any) {}

func normalizeArgs(args []any) (string, []any) {
	if len(args) == 0 {
		return "", nil
	}
	msg := ""
	var kv []any
	switch first := args[0].(type) {
	case string:
		msg = first
	case error:
		msg = first.Error()
		kv = append(kv, "error_0", first)
	default:
		msg = fmt.Sprint(first)
	}
	rest := args[1:]
	errIdx := 1
	for _, arg := range rest {
		if err, ok := arg.(error); ok {
			key := fmt.Sprintf("error_%d", errIdx)
			kv = append(kv, key, err)
			errIdx++
		} else {
			kv = append(kv, arg)
		}
	}
	return msg, kv
}

func (l *Logger) Debugf(format string, args ...any) {
	if !l.slog.Enabled(l.ctx, slog.LevelDebug) {
		return
	}
	caller := getCallerShort(2)
	msg := fmt.Sprintf(format, args...)
	l.slog.Log(l.ctx, slog.LevelDebug, msg, "_caller", caller)
}

func (l *Logger) Infof(format string, args ...any) {
	if !l.slog.Enabled(l.ctx, slog.LevelInfo) {
		return
	}
	caller := getCallerShort(2)
	msg := fmt.Sprintf(format, args...)
	l.slog.Log(l.ctx, slog.LevelInfo, msg, "_caller", caller)
}

func (l *Logger) Warnf(format string, args ...any) {
	if !l.slog.Enabled(l.ctx, slog.LevelWarn) {
		return
	}
	caller := getCallerShort(2)
	msg := fmt.Sprintf(format, args...)
	l.slog.Log(l.ctx, slog.LevelWarn, msg, "_caller", caller)
}

func (l *Logger) Errorf(format string, args ...any) {
	if !l.slog.Enabled(l.ctx, slog.LevelError) {
		return
	}
	caller := getCallerShort(2)
	msg := fmt.Sprintf(format, args...)
	l.slog.Log(l.ctx, slog.LevelError, msg, "_caller", caller)
}

// formatLog 旧底层格式化函数不再使用
func (l *Logger) formatLog(_ slog.Level, _ string, _ ...any) {}

// ---------------- 包级便捷函数，全部调用 FromCtx ----------------
func Debug(ctx context.Context, args ...any) {
	lg := FromCtx(ctx)
	if !lg.slog.Enabled(lg.ctx, slog.LevelDebug) {
		return
	}
	caller := getCallerShort(2)
	msg, kv := normalizeArgs(args)
	kv = append(kv, "_caller", caller)
	lg.slog.Log(lg.ctx, slog.LevelDebug, msg, kv...)
}

func Info(ctx context.Context, args ...any) {
	lg := FromCtx(ctx)
	if !lg.slog.Enabled(lg.ctx, slog.LevelInfo) {
		return
	}
	caller := getCallerShort(2)
	msg, kv := normalizeArgs(args)
	kv = append(kv, "_caller", caller)
	lg.slog.Log(lg.ctx, slog.LevelInfo, msg, kv...)
}

func Warn(ctx context.Context, args ...any) {
	lg := FromCtx(ctx)
	if !lg.slog.Enabled(lg.ctx, slog.LevelWarn) {
		return
	}
	caller := getCallerShort(2)
	msg, kv := normalizeArgs(args)
	kv = append(kv, "_caller", caller)
	lg.slog.Log(lg.ctx, slog.LevelWarn, msg, kv...)
}

func Error(ctx context.Context, args ...any) {
	lg := FromCtx(ctx)
	if !lg.slog.Enabled(lg.ctx, slog.LevelError) {
		return
	}
	caller := getCallerShort(2)
	msg, kv := normalizeArgs(args)
	kv = append(kv, "_caller", caller)
	lg.slog.Log(lg.ctx, slog.LevelError, msg, kv...)
}

func Debugf(ctx context.Context, format string, args ...any) {
	lg := FromCtx(ctx)
	if !lg.slog.Enabled(lg.ctx, slog.LevelDebug) {
		return
	}
	caller := getCallerShort(2)
	msg := fmt.Sprintf(format, args...)
	lg.slog.Log(lg.ctx, slog.LevelDebug, msg, "_caller", caller)
}

func Infof(ctx context.Context, format string, args ...any) {
	lg := FromCtx(ctx)
	if !lg.slog.Enabled(lg.ctx, slog.LevelInfo) {
		return
	}
	caller := getCallerShort(2) // ←这里修复！！
	msg := fmt.Sprintf(format, args...)
	lg.slog.Log(lg.ctx, slog.LevelInfo, msg, "_caller", caller)
}

func Warnf(ctx context.Context, format string, args ...any) {
	lg := FromCtx(ctx)
	if !lg.slog.Enabled(lg.ctx, slog.LevelWarn) {
		return
	}
	caller := getCallerShort(2)
	msg := fmt.Sprintf(format, args...)
	lg.slog.Log(lg.ctx, slog.LevelWarn, msg, "_caller", caller)
}

func Errorf(ctx context.Context, format string, args ...any) {
	lg := FromCtx(ctx)
	if !lg.slog.Enabled(lg.ctx, slog.LevelError) {
		return
	}
	caller := getCallerShort(2)
	msg := fmt.Sprintf(format, args...)
	lg.slog.Log(lg.ctx, slog.LevelError, msg, "_caller", caller)
}

// ---------------- 自定义 TextHandler ----------------
var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type TextHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
	group string
	mu    *sync.Mutex
}

func NewTextHandler(w io.Writer, level slog.Level) *TextHandler {
	return &TextHandler{w: w, level: level, mu: &sync.Mutex{}}
}

func (h *TextHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *TextHandler) Handle(ctx context.Context, r slog.Record) error {
	buf := bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufPool.Put(buf)
	}()
	asctime := r.Time.Format("2006-01-02 15:04:05 -0700")
	levelPad := fmt.Sprintf("%-7s", strings.ToLower(r.Level.String()))
	caller := "-"
	xRequestID := "-"
	var extra []string
	appendAttr := func(a slog.Attr) {
		key := a.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		if key == "request_id" {
			if s := a.Value.String(); s != "" {
				xRequestID = s
			}
			return
		}
		if key == "_caller" {
			caller = a.Value.String()
			return
		}
		extra = append(extra, fmt.Sprintf("%s=%s", key, formatValue(a.Value)))
	}
	for _, a := range h.attrs {
		appendAttr(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		appendAttr(a)
		return true
	})
	if xRequestID == "-" {
		xRequestID = RequestIDFromCtx(ctx)
	}
	extraStr := ""
	if len(extra) > 0 {
		extraStr = " " + strings.Join(extra, " ")
	}
	_, _ = fmt.Fprintf(buf,
		"[%s] [%s] [%s] [%s] %s%s\n",
		asctime, xRequestID, levelPad, caller, r.Message, extraStr,
	)
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *TextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := &TextHandler{
		w:     h.w,
		level: h.level,
		group: h.group,
		mu:    h.mu,
	}
	nh.attrs = make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	nh.attrs = append(nh.attrs, h.attrs...)
	nh.attrs = append(nh.attrs, attrs...)
	return nh
}

func (h *TextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nh := &TextHandler{
		w:     h.w,
		level: h.level,
		mu:    h.mu,
	}
	nh.attrs = make([]slog.Attr, len(h.attrs))
	copy(nh.attrs, h.attrs)
	if h.group != "" {
		nh.group = h.group + "." + name
	} else {
		nh.group = name
	}
	return nh
}

// ---------------- attr 格式化 ----------------
func formatValue(v slog.Value) string {
	v = v.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindGroup:
		parts := make([]string, 0, len(v.Group()))
		for _, a := range v.Group() {
			parts = append(parts, fmt.Sprintf("%s=%s", a.Key, formatValue(a.Value)))
		}
		return "{" + strings.Join(parts, " ") + "}"
	case slog.KindAny:
		if err, ok := v.Any().(error); ok {
			return err.Error()
		}
		return fmt.Sprintf("%v", v.Any())
	default:
		return v.String()
	}
}

// ---------------- 初始化 ----------------
func toSlogLevel(lev string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(lev)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func InitLogger(cfg *cfgtypes.Log) error {
	if cfg == nil {
		cfg = &cfgtypes.Log{}
	}
	var writers []io.Writer
	if cfg.Output.File.Name != "" {
		lj := &lumberjack.Logger{
			Filename:   cfg.Output.File.Name,
			MaxSize:    cfg.Output.File.MaxSize,
			MaxBackups: cfg.Output.File.MaxBackups,
			MaxAge:     cfg.Output.File.MaxAge,
			Compress:   cfg.Output.File.Compress,
			LocalTime:  true,
		}
		if fi, err := os.Stat(lj.Filename); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("logger file stat failed: %w", err)
			}
		} else if fi.IsDir() {
			return fmt.Errorf("logger path is directory, not file: %s", lj.Filename)
		}
		writers = append(writers, lj)
		if cfg.Output.Stdout == "-" {
			writers = append(writers, os.Stdout)
		}
	} else {
		writers = append(writers, os.Stdout)
	}
	mw := io.MultiWriter(writers...)
	handler := NewTextHandler(mw, toSlogLevel(cfg.Level))
	slog.SetDefault(slog.New(handler))
	return nil
}
