// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !noasm,!gccgo,!safe

#include "textflag.h"

// func L1NormInc(x []float64, n, incX int) (sum float64)
TEXT ·L1NormInc(SB), NOSPLIT, $0
	MOVQ  x_base+0(FP), SI // SI = &x
	MOVQ  n+24(FP), CX     // CX = n
	MOVQ  incX+32(FP), AX  // AX =  increment * sizeof( float64 )
	SHLQ  $3, AX
	MOVQ  AX, DX           // DX = AX * 3
	IMULQ $3, DX
	PXOR  X0, X0           // p_sum_i = 0
	PXOR  X1, X1
	PXOR  X2, X2
	PXOR  X3, X3
	PXOR  X4, X4
	PXOR  X5, X5
	PXOR  X6, X6
	PXOR  X7, X7
	CMPQ  CX, $0           // if CX == 0 { return 0 }
	JE    absum_end
	MOVQ  CX, BX
	ANDQ  $7, BX           // BX = n % 8
	SHRQ  $3, CX           // CX = floor( n / 8 )
	JZ    absum_tail_start // if CX == 0 { goto absum_tail_start }

absum_loop: // do {
	// p_sum = max( p_sum + x[i], p_sum - x[i] )
	MOVSD  (SI), X8        // X_i[0] = x[i]
	MOVSD  (SI)(AX*1), X9
	MOVSD  (SI)(AX*2), X10
	MOVSD  (SI)(DX*1), X11
	LEAQ   (SI)(AX*4), SI  // SI = SI + 4
	MOVHPD (SI), X8        // X_i[1] = x[i+4]
	MOVHPD (SI)(AX*1), X9
	MOVHPD (SI)(AX*2), X10
	MOVHPD (SI)(DX*1), X11
	ADDPD  X8, X0          // p_sum_i += X_i  ( positive values )
	ADDPD  X9, X2
	ADDPD  X10, X4
	ADDPD  X11, X6
	SUBPD  X8, X1          // p_sum_(i+1) -= X_i  ( negative values )
	SUBPD  X9, X3
	SUBPD  X10, X5
	SUBPD  X11, X7
	MAXPD  X1, X0          // p_sum_i = max( p_sum_i, p_sum_(i+1) )
	MAXPD  X3, X2
	MAXPD  X5, X4
	MAXPD  X7, X6
	MOVAPS X0, X1          // p_sum_(i+1) = p_sum_i
	MOVAPS X2, X3
	MOVAPS X4, X5
	MOVAPS X6, X7
	LEAQ   (SI)(AX*4), SI  // SI = SI + 4
	LOOP   absum_loop      // } while --CX > 0

	// p_sum_0 = \sum_{i=1}^{3}( p_sum_(i*2) )
	ADDPD X3, X0
	ADDPD X5, X7
	ADDPD X7, X0

	// p_sum_0[0] = p_sum_0[0] + p_sum_0[1]
	MOVAPS X0, X1
	SHUFPD $0x3, X0, X0 // lower( p_sum_0 ) = upper( p_sum_0 )
	ADDSD  X1, X0
	CMPQ   BX, $0
	JE     absum_end    // if BX == 0 { goto absum_end }

absum_tail_start: // Reset loop registers
	MOVQ  BX, CX // Loop counter:  CX = BX
	XORPS X8, X8 // X_8 = 0

absum_tail: // do {
	// p_sum += max( p_sum + x[i], p_sum - x[i] )
	MOVSD (SI), X8   // X_8 = x[i]
	MOVSD X0, X1     // p_sum_1 = p_sum_0
	ADDSD X8, X0     // p_sum_0 += X_8
	SUBSD X8, X1     // p_sum_1 -= X_8
	MAXSD X1, X0     // p_sum_0 = max( p_sum_0, p_sum_1 )
	ADDQ  AX, SI     // i++
	LOOP  absum_tail // } while --CX > 0

absum_end: // return p_sum_0
	MOVSD X0, sum+40(FP)
	RET
