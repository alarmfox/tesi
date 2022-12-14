// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !noasm,!gccgo,!safe

#include "textflag.h"

// MOVSHDUP X3, X2
#define MOVSHDUP_X3_X2 BYTE $0xF3; BYTE $0x0F; BYTE $0x16; BYTE $0xD3
// MOVSLDUP X3, X3
#define MOVSLDUP_X3_X3 BYTE $0xF3; BYTE $0x0F; BYTE $0x12; BYTE $0xDB
// ADDSUBPS X2, X3
#define ADDSUBPS_X2_X3 BYTE $0xF2; BYTE $0x0F; BYTE $0xD0; BYTE $0xDA

// MOVSHDUP X5, X4
#define MOVSHDUP_X5_X4 BYTE $0xF3; BYTE $0x0F; BYTE $0x16; BYTE $0xE5
// MOVSLDUP X5, X5
#define MOVSLDUP_X5_X5 BYTE $0xF3; BYTE $0x0F; BYTE $0x12; BYTE $0xED
// ADDSUBPS X4, X5
#define ADDSUBPS_X4_X5 BYTE $0xF2; BYTE $0x0F; BYTE $0xD0; BYTE $0xEC

// MOVSHDUP X7, X6
#define MOVSHDUP_X7_X6 BYTE $0xF3; BYTE $0x0F; BYTE $0x16; BYTE $0xF7
// MOVSLDUP X7, X7
#define MOVSLDUP_X7_X7 BYTE $0xF3; BYTE $0x0F; BYTE $0x12; BYTE $0xFF
// ADDSUBPS X6, X7
#define ADDSUBPS_X6_X7 BYTE $0xF2; BYTE $0x0F; BYTE $0xD0; BYTE $0xFE

// MOVSHDUP X9, X8
#define MOVSHDUP_X9_X8 BYTE $0xF3; BYTE $0x45; BYTE $0x0F; BYTE $0x16; BYTE $0xC1
// MOVSLDUP X9, X9
#define MOVSLDUP_X9_X9 BYTE $0xF3; BYTE $0x45; BYTE $0x0F; BYTE $0x12; BYTE $0xC9
// ADDSUBPS X8, X9
#define ADDSUBPS_X8_X9 BYTE $0xF2; BYTE $0x45; BYTE $0x0F; BYTE $0xD0; BYTE $0xC8

// func AxpyIncTo(dst []complex64, incDst, idst uintptr, alpha complex64, x, y []complex64, n, incX, incY, ix, iy uintptr)
TEXT ·AxpyIncTo(SB), NOSPLIT, $0
	MOVQ   dst_base+0(FP), DI // DI = &dst
	MOVQ   x_base+48(FP), SI  // SI = &x
	MOVQ   y_base+72(FP), DX  // DX = &y
	MOVQ   n+96(FP), CX       // CX = n
	CMPQ   CX, $0             // if n==0 { return }
	JE     axpyi_end
	MOVQ   ix+120(FP), R8     // Load the first index
	MOVQ   iy+128(FP), R9
	MOVQ   idst+32(FP), R10
	LEAQ   (SI)(R8*8), SI     // SI = &(x[ix])
	LEAQ   (DX)(R9*8), DX     // DX = &(y[iy])
	LEAQ   (DI)(R10*8), DI    // DI = &(dst[idst])
	MOVQ   incX+104(FP), R8   // Incrementors*8 for easy iteration (ADDQ)
	SHLQ   $3, R8
	MOVQ   incY+112(FP), R9
	SHLQ   $3, R9
	MOVQ   incDst+24(FP), R10
	SHLQ   $3, R10
	MOVSD  alpha+40(FP), X0   // X0 = { 0, 0, imag(a), real(a) }
	MOVAPS X0, X1
	SHUFPS $0x11, X1, X1      // X1 = { 0, 0, real(a), imag(a) }
	MOVAPS X0, X10            // Copy X0 and X1 for pipelining
	MOVAPS X1, X11
	MOVQ   CX, BX
	ANDQ   $3, CX             // CX = n % 4
	SHRQ   $2, BX             // BX = floor( n / 4 )
	JZ     axpyi_tail         // if BX == 0 { goto axpyi_tail }

axpyi_loop: // do {
	MOVSD (SI), X3       // X_i = { imag(x[i]), real(x[i]) }
	MOVSD (SI)(R8*1), X5
	LEAQ  (SI)(R8*2), SI // SI = &(SI[incX*2])
	MOVSD (SI), X7
	MOVSD (SI)(R8*1), X9

	// X_(i-1) = { imag(x[i]), imag(x[i]) }
	MOVSHDUP_X3_X2
	MOVSHDUP_X5_X4
	MOVSHDUP_X7_X6
	MOVSHDUP_X9_X8

	// X_i = { real(x[i]), real(x[i]) }
	MOVSLDUP_X3_X3
	MOVSLDUP_X5_X5
	MOVSLDUP_X7_X7
	MOVSLDUP_X9_X9

	// X_(i-1) = {  real(a) * imag(x[i]),   imag(a) * imag(x[i]) }
	// X_i     = {  imag(a) * real(x[i]),   real(a) * real(x[i])  }
	MULPS X1, X2
	MULPS X0, X3
	MULPS X11, X4
	MULPS X10, X5
	MULPS X1, X6
	MULPS X0, X7
	MULPS X11, X8
	MULPS X10, X9

	// X_i = {
	//	imag(result[i]):   imag(a)*real(x[i]) + real(a)*imag(x[i]),
	//	real(result[i]):   real(a)*real(x[i]) - imag(a)*imag(x[i]),
	//  }
	ADDSUBPS_X2_X3
	ADDSUBPS_X4_X5
	ADDSUBPS_X6_X7
	ADDSUBPS_X8_X9

	// X_i = { imag(result[i]) + imag(y[i]), real(result[i]) + real(y[i]) }
	MOVSD (DX), X2
	MOVSD (DX)(R9*1), X4
	LEAQ  (DX)(R9*2), DX // DX = &(DX[incY*2])
	MOVSD (DX), X6
	MOVSD (DX)(R9*1), X8
	ADDPS X2, X3
	ADDPS X4, X5
	ADDPS X6, X7
	ADDPS X8, X9

	MOVSD X3, (DI)        // y[i] = X_i
	MOVSD X5, (DI)(R10*1)
	LEAQ  (DI)(R10*2), DI // DI = &(DI[incDst])
	MOVSD X7, (DI)
	MOVSD X9, (DI)(R10*1)
	LEAQ  (SI)(R8*2), SI  // SI = &(SI[incX*2])
	LEAQ  (DX)(R9*2), DX  // DX = &(DX[incY*2])
	LEAQ  (DI)(R10*2), DI // DI = &(DI[incDst])
	DECQ  BX
	JNZ   axpyi_loop      // } while --BX > 0
	CMPQ  CX, $0          // if CX == 0 { return }
	JE    axpyi_end

axpyi_tail:
	MOVSD (SI), X3 // X_i     = { imag(x[i]), real(x[i]) }
	MOVSHDUP_X3_X2 // X_(i-1) = { imag(x[i]), imag(x[i]) }
	MOVSLDUP_X3_X3 // X_i     = { real(x[i]), real(x[i]) }

	// X_i     = { imag(a) * real(x[i]),  real(a) * real(x[i]) }
	// X_(i-1) = { real(a) * imag(x[i]),  imag(a) * imag(x[i]) }
	MULPS X1, X2
	MULPS X0, X3

	// X_i = {
	//	imag(result[i]):   imag(a)*real(x[i]) + real(a)*imag(x[i]),
	//	real(result[i]):   real(a)*real(x[i]) - imag(a)*imag(x[i]),
	//  }
	ADDSUBPS_X2_X3

	// X_i = { imag(result[i]) + imag(y[i]),  real(result[i]) + real(y[i])  }
	MOVSD (DX), X4
	ADDPS X4, X3
	MOVSD X3, (DI)   // y[i] = X_i
	ADDQ  R8, SI     // SI += incX
	ADDQ  R9, DX     // DX += incY
	ADDQ  R10, DI    // DI += incDst
	LOOP  axpyi_tail // } while --CX > 0

axpyi_end:
	RET
