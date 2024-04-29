// Copyright ©2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// Pre-defined palettes based on the 2014 Material UI color palettes.
var (
	RedPalette = Palette{
		RGB(255, 235, 238), // 0
		RGB(255, 205, 210), // 1
		RGB(239, 154, 154), // 2
		RGB(229, 115, 115), // 3
		RGB(239, 83, 80),   // 4
		RGB(244, 67, 54),   // 5
		RGB(229, 57, 53),   // 6
		RGB(211, 47, 47),   // 7
		RGB(198, 40, 40),   // 8
		RGB(183, 28, 28),   // 9
	}
	PinkPalette = Palette{
		RGB(252, 228, 236), // 0
		RGB(248, 187, 208), // 1
		RGB(244, 143, 177), // 2
		RGB(240, 98, 146),  // 3
		RGB(236, 64, 122),  // 4
		RGB(233, 30, 99),   // 5
		RGB(216, 27, 96),   // 6
		RGB(194, 24, 91),   // 7
		RGB(173, 20, 87),   // 8
		RGB(136, 14, 79),   // 9
	}
	PurplePalette = Palette{
		RGB(243, 229, 245), // 0
		RGB(225, 190, 231), // 1
		RGB(206, 147, 216), // 2
		RGB(186, 104, 200), // 3
		RGB(171, 71, 188),  // 4
		RGB(156, 39, 176),  // 5
		RGB(142, 36, 170),  // 6
		RGB(123, 31, 162),  // 7
		RGB(106, 27, 154),  // 8
		RGB(74, 20, 140),   // 9
	}
	DeepPurplePalette = Palette{
		RGB(237, 231, 246), // 0
		RGB(209, 196, 233), // 1
		RGB(179, 157, 219), // 2
		RGB(149, 117, 205), // 3
		RGB(126, 87, 194),  // 4
		RGB(103, 58, 183),  // 5
		RGB(94, 53, 177),   // 6
		RGB(81, 45, 168),   // 7
		RGB(69, 39, 160),   // 8
		RGB(49, 27, 146),   // 9
	}
	IndigoPalette = Palette{
		RGB(232, 234, 246), // 0
		RGB(197, 202, 233), // 1
		RGB(159, 168, 218), // 2
		RGB(121, 134, 203), // 3
		RGB(92, 107, 192),  // 4
		RGB(63, 81, 181),   // 5
		RGB(57, 73, 171),   // 6
		RGB(48, 63, 159),   // 7
		RGB(40, 53, 147),   // 8
		RGB(26, 35, 126),   // 9
	}
	BluePalette = Palette{
		RGB(227, 242, 253), // 0
		RGB(187, 222, 251), // 1
		RGB(144, 202, 249), // 2
		RGB(100, 181, 246), // 3
		RGB(66, 165, 245),  // 4
		RGB(33, 150, 243),  // 5
		RGB(30, 136, 229),  // 6
		RGB(25, 118, 210),  // 7
		RGB(21, 101, 192),  // 8
		RGB(13, 71, 161),   // 9
	}
	LightBluePalette = Palette{
		RGB(225, 245, 254), // 0
		RGB(179, 229, 252), // 1
		RGB(129, 212, 250), // 2
		RGB(79, 195, 247),  // 3
		RGB(41, 182, 246),  // 4
		RGB(3, 169, 244),   // 5
		RGB(3, 155, 229),   // 6
		RGB(2, 136, 209),   // 7
		RGB(2, 119, 189),   // 8
		RGB(1, 87, 155),    // 9
	}
	CyanPalette = Palette{
		RGB(224, 247, 250), // 0
		RGB(178, 235, 242), // 1
		RGB(128, 222, 234), // 2
		RGB(77, 208, 225),  // 3
		RGB(38, 198, 218),  // 4
		RGB(0, 188, 212),   // 5
		RGB(0, 172, 193),   // 6
		RGB(0, 151, 167),   // 7
		RGB(0, 131, 143),   // 8
		RGB(0, 96, 100),    // 9
	}
	TealPalette = Palette{
		RGB(224, 242, 241), // 0
		RGB(178, 223, 219), // 1
		RGB(128, 203, 196), // 2
		RGB(77, 182, 172),  // 3
		RGB(38, 166, 154),  // 4
		RGB(0, 150, 136),   // 5
		RGB(0, 137, 123),   // 6
		RGB(0, 121, 107),   // 7
		RGB(0, 105, 92),    // 8
		RGB(0, 77, 64),     // 9
	}
	GreenPalette = Palette{
		RGB(232, 245, 233), // 0
		RGB(200, 230, 201), // 1
		RGB(165, 214, 167), // 2
		RGB(129, 199, 132), // 3
		RGB(102, 187, 106), // 4
		RGB(76, 175, 80),   // 5
		RGB(67, 160, 71),   // 6
		RGB(56, 142, 60),   // 7
		RGB(46, 125, 50),   // 8
		RGB(27, 94, 32),    // 9
	}
	LightGreenPalette = Palette{
		RGB(241, 248, 233), // 0
		RGB(220, 237, 200), // 1
		RGB(197, 225, 165), // 2
		RGB(174, 213, 129), // 3
		RGB(156, 204, 101), // 4
		RGB(139, 195, 74),  // 5
		RGB(124, 179, 66),  // 6
		RGB(104, 159, 56),  // 7
		RGB(85, 139, 47),   // 8
		RGB(51, 105, 30),   // 9
	}
	LimePalette = Palette{
		RGB(249, 251, 231), // 0
		RGB(240, 244, 195), // 1
		RGB(230, 238, 156), // 2
		RGB(220, 231, 117), // 3
		RGB(212, 225, 87),  // 4
		RGB(205, 220, 57),  // 5
		RGB(192, 202, 51),  // 6
		RGB(175, 180, 43),  // 7
		RGB(158, 157, 36),  // 8
		RGB(130, 119, 23),  // 9
	}
	YellowPalette = Palette{
		RGB(255, 253, 231), // 0
		RGB(255, 249, 196), // 1
		RGB(255, 245, 157), // 2
		RGB(255, 241, 118), // 3
		RGB(255, 238, 88),  // 4
		RGB(255, 235, 59),  // 5
		RGB(253, 216, 53),  // 6
		RGB(251, 192, 45),  // 7
		RGB(249, 168, 37),  // 8
		RGB(245, 127, 23),  // 9
	}
	AmberPalette = Palette{
		RGB(255, 248, 225), // 0
		RGB(255, 236, 179), // 1
		RGB(255, 224, 130), // 2
		RGB(255, 213, 79),  // 3
		RGB(255, 202, 40),  // 4
		RGB(255, 193, 7),   // 5
		RGB(255, 179, 0),   // 6
		RGB(255, 160, 0),   // 7
		RGB(255, 143, 0),   // 8
		RGB(255, 111, 0),   // 9
	}
	OrangePalette = Palette{
		RGB(255, 243, 224), // 0
		RGB(255, 224, 178), // 1
		RGB(255, 204, 128), // 2
		RGB(255, 183, 77),  // 3
		RGB(255, 167, 38),  // 4
		RGB(255, 152, 0),   // 5
		RGB(251, 140, 0),   // 6
		RGB(245, 124, 0),   // 7
		RGB(239, 108, 0),   // 8
		RGB(230, 81, 0),    // 9
	}
	DeepOrangePalette = Palette{
		RGB(251, 233, 231), // 0
		RGB(255, 204, 188), // 1
		RGB(255, 171, 145), // 2
		RGB(255, 138, 101), // 3
		RGB(255, 112, 67),  // 4
		RGB(255, 87, 34),   // 5
		RGB(244, 81, 30),   // 6
		RGB(230, 74, 25),   // 7
		RGB(216, 67, 21),   // 8
		RGB(191, 54, 12),   // 9
	}
	BrownPalette = Palette{
		RGB(239, 235, 233), // 0
		RGB(215, 204, 200), // 1
		RGB(188, 170, 164), // 2
		RGB(161, 136, 127), // 3
		RGB(141, 110, 99),  // 4
		RGB(121, 85, 72),   // 5
		RGB(109, 76, 65),   // 6
		RGB(93, 64, 55),    // 7
		RGB(78, 52, 46),    // 8
		RGB(62, 39, 35),    // 9
	}
	GreyPalette = Palette{
		RGB(250, 250, 250), // 0
		RGB(245, 245, 245), // 1
		RGB(238, 238, 238), // 2
		RGB(224, 224, 224), // 3
		RGB(189, 189, 189), // 4
		RGB(158, 158, 158), // 5
		RGB(117, 117, 117), // 6
		RGB(97, 97, 97),    // 7
		RGB(66, 66, 66),    // 8
		RGB(33, 33, 33),    // 9
	}
	BlueGreyPalette = Palette{
		RGB(236, 239, 241), // 0
		RGB(207, 216, 220), // 1
		RGB(176, 190, 197), // 2
		RGB(144, 164, 174), // 3
		RGB(120, 144, 156), // 4
		RGB(96, 125, 139),  // 5
		RGB(84, 110, 122),  // 6
		RGB(69, 90, 100),   // 7
		RGB(55, 71, 79),    // 8
		RGB(38, 50, 56),    // 9
	}
)

// Palette is a collection of similar colors, ranging from lightest (0) to darkest (9).
type Palette [10]Color
