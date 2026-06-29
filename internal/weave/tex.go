//line lit/weave.w:1171
package weave

//line lit/weave.w:1173
import (
//line lit/weave.w:1174
	"fmt"
//line lit/weave.w:1175
	"strings"
//line lit/weave.w:1176
)

//line lit/weave.w:1178
// TeX escaping. Three contexts need different treatment:
//line lit/weave.w:1179
//
//line lit/weave.w:1180
//   - identifiers/keywords: only '_' is troublesome (\_ works in text mode);
//line lit/weave.w:1181
//   - typewriter text (strings, comments): every TeX special is emitted as a
//line lit/weave.w:1182
//     \charNN code so it prints literally regardless of the current font;
//line lit/weave.w:1183
//   - prose names and math operators: text-mode / math-mode safe sequences.

// escIdent escapes an identifier or keyword for text mode.
//
//line lit/weave.w:1187
//line lit/weave.w:1188
func escIdent(s string) string {
//line lit/weave.w:1189
	return strings.ReplaceAll(s, "_", "\\_")
//line lit/weave.w:1190
}

// escTT escapes arbitrary text for a typewriter (\tt) box. Specials become
// \charNN so braces, backslashes, etc. survive.
//
//line lit/weave.w:1194
//line lit/weave.w:1195
//line lit/weave.w:1196
func escTT(s string) string {
//line lit/weave.w:1197
	var b strings.Builder
//line lit/weave.w:1198
	for i := 0; i < len(s); i++ {
//line lit/weave.w:1199
		c := s[i]
//line lit/weave.w:1200
		switch c {
//line lit/weave.w:1201
		case '\\', '{', '}', '$', '&', '#', '%', '^', '_', '~':
//line lit/weave.w:1202
			fmt.Fprintf(&b, "\\char%d ", c)
//line lit/weave.w:1203
		case ' ':
//line lit/weave.w:1204
			// a visible space (\GSP): cweb prints the blanks inside a string with a
//line lit/weave.w:1205
			// space glyph, slot 32 of the typewriter font.
//line lit/weave.w:1206
			b.WriteString("\\GSP ")
//line lit/weave.w:1207
		default:
//line lit/weave.w:1208
			b.WriteByte(c)
//line lit/weave.w:1209
		}
//line lit/weave.w:1210
	}
//line lit/weave.w:1211
	return b.String()
//line lit/weave.w:1212
}

// escMathOp encodes an operator/punctuation run so it is safe inside math mode.
//
//line lit/weave.w:1216
//line lit/weave.w:1217
func escMathOp(s string) string {
//line lit/weave.w:1218
	var b strings.Builder
//line lit/weave.w:1219
	for i := 0; i < len(s); i++ {
//line lit/weave.w:1220
		switch c := s[i]; c {
//line lit/weave.w:1221
		case '{':
//line lit/weave.w:1222
			b.WriteString("\\{")
//line lit/weave.w:1223
		case '}':
//line lit/weave.w:1224
			b.WriteString("\\}")
//line lit/weave.w:1225
		case '&':
//line lit/weave.w:1226
			b.WriteString("\\&")
//line lit/weave.w:1227
		case '#':
//line lit/weave.w:1228
			b.WriteString("\\#")
//line lit/weave.w:1229
		case '%':
//line lit/weave.w:1230
			b.WriteString("\\%")
//line lit/weave.w:1231
		case '$':
//line lit/weave.w:1232
			b.WriteString("\\$")
//line lit/weave.w:1233
		case '_':
//line lit/weave.w:1234
			b.WriteString("\\_")
//line lit/weave.w:1235
		case '^':
//line lit/weave.w:1236
			b.WriteString("\\char94 ")
//line lit/weave.w:1237
		case '~':
//line lit/weave.w:1238
			b.WriteString("\\char126 ")
//line lit/weave.w:1239
		case '|':
//line lit/weave.w:1240
			b.WriteString("\\char124 ")
//line lit/weave.w:1241
		case '\\':
//line lit/weave.w:1242
			b.WriteString("\\backslash ")
//line lit/weave.w:1243
		default:
//line lit/weave.w:1244
			b.WriteByte(c)
//line lit/weave.w:1245
		}
//line lit/weave.w:1246
	}
//line lit/weave.w:1247
	return b.String()
//line lit/weave.w:1248
}

// renderOp typesets a Go operator token as a single tight math atom (no math
// spacing of its own), using real math symbols where they exist. Inter-token
// spacing is supplied by the surrounding source whitespace, so the result
// reproduces gofmt's spacing exactly and the unary/binary distinction for *, &,
// etc. needs no grammar analysis.
//
//line lit/weave.w:1254
//line lit/weave.w:1255
//line lit/weave.w:1256
//line lit/weave.w:1257
//line lit/weave.w:1258
//line lit/weave.w:1259
func renderOp(s string) string {
//line lit/weave.w:1260
	switch s {
//line lit/weave.w:1261
	case "<=":
//line lit/weave.w:1262
		return "\\mathord{\\leq}"
//line lit/weave.w:1263
	case ">=":
//line lit/weave.w:1264
		return "\\mathord{\\geq}"
//line lit/weave.w:1265
	case "!=":
//line lit/weave.w:1266
		return "\\mathord{\\neq}"
//line lit/weave.w:1267
	case "==":
//line lit/weave.w:1268
		return "\\mathord{\\equiv}" // equality test, as cweb (an equivalence sign)
//line lit/weave.w:1269
	case "!":
//line lit/weave.w:1270
		return "\\mathord{\\lnot}" // logical not, as cweb (a negation sign)
//line lit/weave.w:1271
	case "&&":
//line lit/weave.w:1272
		return "\\mathord{\\land}" // logical and, as cweb (a wedge)
//line lit/weave.w:1273
	case "||":
//line lit/weave.w:1274
		return "\\mathord{\\lor}" // logical or, as cweb (a vee)
//line lit/weave.w:1275
	case "<-":
//line lit/weave.w:1276
		return "\\mathord{\\leftarrow}"
//line lit/weave.w:1277
	case "^":
//line lit/weave.w:1278
		return "\\mathord{\\oplus}" // bitwise xor, as cweb (a circled plus)
//line lit/weave.w:1279
	case "^=":
//line lit/weave.w:1280
		return "\\mathord{\\oplus}\\mathord{=}" // xor-assign: ^ is a circled plus too
//line lit/weave.w:1281
	case "&^":
//line lit/weave.w:1282
		return "\\mathord{\\&}\\mathord{\\oplus}" // bit clear (and-not): ^ as circled plus
//line lit/weave.w:1283
	case "&^=":
//line lit/weave.w:1284
		return "\\mathord{\\&}\\mathord{\\oplus}\\mathord{=}" // and-not-assign
//line lit/weave.w:1285
	case "<<":
//line lit/weave.w:1286
		return "\\mathord{\\ll}" // left shift, as cweb (a tight double angle)
//line lit/weave.w:1287
	case ">>":
//line lit/weave.w:1288
		return "\\mathord{\\gg}" // right shift
//line lit/weave.w:1289
	case "<<=":
//line lit/weave.w:1290
		return "\\mathord{\\ll}\\mathord{=}"
//line lit/weave.w:1291
	case ">>=":
//line lit/weave.w:1292
		return "\\mathord{\\gg}\\mathord{=}"
//line lit/weave.w:1293
	case "...":
//line lit/weave.w:1294
		return "\\mathord{\\ldots}"
//line lit/weave.w:1295
	case "[]":
//line lit/weave.w:1296
		// empty slice/array brackets: a thin space keeps them from jamming
//line lit/weave.w:1297
		return "\\mathord{[}\\,\\mathord{]}"
//line lit/weave.w:1298
	case "{}":
//line lit/weave.w:1299
		// empty braces (struct{}, interface{}, T{}): likewise a thin space
//line lit/weave.w:1300
		return "\\mathord{\\{}\\,\\mathord{\\}}"
//line lit/weave.w:1301
	}
//line lit/weave.w:1302
	if len(s) == 1 {
//line lit/weave.w:1303
		return "\\mathord{" + escMathOp(s) + "}"
//line lit/weave.w:1304
	}
//line lit/weave.w:1305
	return tightMathOp(s)
//line lit/weave.w:1306
}

// tightMathOp encodes each character of an operator as an ordinary atom, so that
// e.g. "==" or "<<" prints with the characters adjacent rather than spaced.
//
//line lit/weave.w:1311
//line lit/weave.w:1312
//line lit/weave.w:1313
func tightMathOp(s string) string {
//line lit/weave.w:1314
	var b strings.Builder
//line lit/weave.w:1315
	for i := 0; i < len(s); i++ {
//line lit/weave.w:1316
		b.WriteString("\\mathord{")
//line lit/weave.w:1317
		b.WriteString(escMathOp(s[i : i+1]))
//line lit/weave.w:1318
		b.WriteString("}")
//line lit/weave.w:1319
	}
//line lit/weave.w:1320
	return b.String()
//line lit/weave.w:1321
}

// escProse escapes text for ordinary roman text mode (used for section names).
//
//line lit/weave.w:1325
//line lit/weave.w:1326
func escProse(s string) string {
//line lit/weave.w:1327
	var b strings.Builder
//line lit/weave.w:1328
	for i := 0; i < len(s); i++ {
//line lit/weave.w:1329
		switch c := s[i]; c {
//line lit/weave.w:1330
		case '_':
//line lit/weave.w:1331
			b.WriteString("\\_")
//line lit/weave.w:1332
		case '&':
//line lit/weave.w:1333
			b.WriteString("\\&")
//line lit/weave.w:1334
		case '#':
//line lit/weave.w:1335
			b.WriteString("\\#")
//line lit/weave.w:1336
		case '%':
//line lit/weave.w:1337
			b.WriteString("\\%")
//line lit/weave.w:1338
		case '$':
//line lit/weave.w:1339
			b.WriteString("\\$")
//line lit/weave.w:1340
		case '{':
//line lit/weave.w:1341
			b.WriteString("$\\{$")
//line lit/weave.w:1342
		case '}':
//line lit/weave.w:1343
			b.WriteString("$\\}$")
//line lit/weave.w:1344
		case '\\':
//line lit/weave.w:1345
			b.WriteString("$\\backslash$")
//line lit/weave.w:1346
		case '^':
//line lit/weave.w:1347
			b.WriteString("\\^{}")
//line lit/weave.w:1348
		case '~':
//line lit/weave.w:1349
			b.WriteString("\\~{}")
//line lit/weave.w:1350
		case '<':
//line lit/weave.w:1351
			b.WriteString("$<$") // cmr (OT1) has no < glyph; use math
//line lit/weave.w:1352
		case '>':
//line lit/weave.w:1353
			b.WriteString("$>$") // likewise for >
//line lit/weave.w:1354
		case '|':
//line lit/weave.w:1355
			b.WriteString("$\\vert$")
//line lit/weave.w:1356
		default:
//line lit/weave.w:1357
			b.WriteByte(c)
//line lit/weave.w:1358
		}
//line lit/weave.w:1359
	}
//line lit/weave.w:1360
	return b.String()
//line lit/weave.w:1361
}

// escComment escapes a comment for roman text mode, but passes a $...$ span
// through unescaped so TeX math works inside comments (as in cweb).
//
//line lit/weave.w:1367
//line lit/weave.w:1368
//line lit/weave.w:1369
func escComment(s string) string {
//line lit/weave.w:1370
	var b strings.Builder
//line lit/weave.w:1371
	for i := 0; i < len(s); {
//line lit/weave.w:1372
		if s[i] == '$' {
//line lit/weave.w:1373
			if k := strings.IndexByte(s[i+1:], '$'); k >= 0 {
//line lit/weave.w:1374
				j := i + 1 + k
//line lit/weave.w:1375
				b.WriteString(s[i : j+1]) // the $...$ math span, verbatim
//line lit/weave.w:1376
				i = j + 1
//line lit/weave.w:1377
				continue
//line lit/weave.w:1378
			}
//line lit/weave.w:1379
		}
//line lit/weave.w:1380
		b.WriteString(escProse(s[i : i+1]))
//line lit/weave.w:1381
		i++
//line lit/weave.w:1382
	}
//line lit/weave.w:1383
	return b.String()
//line lit/weave.w:1384
}
