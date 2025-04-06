package unison

import (
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/unison/enums/slant"
	"github.com/richardwilkes/unison/enums/spacing"
	"github.com/richardwilkes/unison/enums/weight"
)

// FontPanel provides a standard panel for selecting a font.
type FontPanel struct {
	fontSizeField        *Field
	fontFamilyPopup      *PopupMenu[string]
	fontWeightPopup      *PopupMenu[weight.Enum]
	fontSlantPopup       *PopupMenu[slant.Enum]
	fontSpacingPopup     *PopupMenu[spacing.Enum]
	FontModifiedCallback func(fd FontDescriptor)
	fontDescriptor       FontDescriptor
	Panel
	DefaultFontSize float32
	MinFontSize     float32
	MaxFontSize     float32
}

// NewFontPanel creates a new FontPanel.
func NewFontPanel() *FontPanel {
	p := &FontPanel{
		fontDescriptor:  SystemFont.Descriptor(),
		DefaultFontSize: 10,
		MinFontSize:     4,
		MaxFontSize:     256,
	}
	p.Self = p

	p.fontSizeField = NewField()
	p.fontSizeField.SetText(formatFloat32(p.fontDescriptor.Size))
	p.fontSizeField.Watermark = formatFloat32(p.DefaultFontSize)
	p.fontSizeField.MinimumTextWidth = 30
	p.fontSizeField.ValidateCallback = p.DefaultValidate
	p.fontSizeField.KeyDownCallback = p.DefaultKeyDown
	UninstallFocusBorders(p.fontSizeField, p.fontSizeField)
	p.fontSizeField.LostFocusCallback = p.DefaultFocusLost
	InstallDefaultFieldBorder(p.fontSizeField, p.fontSizeField)
	p.AddChild(p.fontSizeField)

	p.fontFamilyPopup = NewPopupMenu[string]()
	p.fontFamilyPopup.AddItem(FontFamilies()...)
	p.fontFamilyPopup.Select(p.fontDescriptor.Family)
	p.fontFamilyPopup.SelectionChangedCallback = func(popup *PopupMenu[string]) {
		if family, ok := popup.Selected(); ok {
			if family != p.fontDescriptor.Family {
				p.fontDescriptor.Family = family
				p.adjustForCurrentFontFamily()
				p.fontModified()
			}
		}
	}
	p.AddChild(p.fontFamilyPopup)

	p.fontWeightPopup = NewPopupMenu[weight.Enum]()
	p.fontWeightPopup.AddItem(weight.All...)
	p.fontWeightPopup.Select(p.fontDescriptor.Weight)
	p.fontWeightPopup.SelectionChangedCallback = func(popup *PopupMenu[weight.Enum]) {
		if value, ok := popup.Selected(); ok {
			if value != p.fontDescriptor.Weight {
				p.fontDescriptor.Weight = value
				p.adjustForCurrentFontFamily()
				p.fontModified()
			}
		}
	}
	p.AddChild(p.fontWeightPopup)

	p.fontSlantPopup = NewPopupMenu[slant.Enum]()
	p.fontSlantPopup.AddItem(slant.All...)
	p.fontSlantPopup.Select(p.fontDescriptor.Slant)
	p.fontSlantPopup.SelectionChangedCallback = func(popup *PopupMenu[slant.Enum]) {
		if value, ok := popup.Selected(); ok {
			if value != p.fontDescriptor.Slant {
				p.fontDescriptor.Slant = value
				p.adjustForCurrentFontFamily()
				p.fontModified()
			}
		}
	}
	p.AddChild(p.fontSlantPopup)

	p.fontSpacingPopup = NewPopupMenu[spacing.Enum]()
	p.fontSpacingPopup.AddItem(spacing.All...)
	p.fontSpacingPopup.Select(p.fontDescriptor.Spacing)
	p.fontSpacingPopup.SelectionChangedCallback = func(popup *PopupMenu[spacing.Enum]) {
		if value, ok := popup.Selected(); ok {
			if value != p.fontDescriptor.Spacing {
				p.fontDescriptor.Spacing = value
				p.adjustForCurrentFontFamily()
				p.fontModified()
			}
		}
	}
	p.AddChild(p.fontSpacingPopup)

	p.adjustForCurrentFontFamily()

	p.SetLayout(&FlexLayout{
		Columns:  len(p.Children()),
		HSpacing: StdHSpacing,
	})
	return p
}

// FontDescriptor returns the font descriptor.
func (p *FontPanel) FontDescriptor() FontDescriptor {
	return p.fontDescriptor
}

// SetFontDescriptor sets the font descriptor.
func (p *FontPanel) SetFontDescriptor(fd FontDescriptor) {
	if fd.Size < p.MinFontSize {
		fd.Size = p.MinFontSize
	} else if fd.Size > p.MaxFontSize {
		fd.Size = p.MaxFontSize
	}
	savedCallback := p.FontModifiedCallback
	p.FontModifiedCallback = nil
	p.fontDescriptor = fd
	p.fontFamilyPopup.Select(p.fontDescriptor.Family)
	p.fontSizeField.SetText(formatFloat32(fd.Size))
	p.adjustForCurrentFontFamily()
	p.FontModifiedCallback = savedCallback
	p.fontModified()
}

// DefaultValidate provides the default validation for the font size field.
func (p *FontPanel) DefaultValidate() bool {
	_, valid := p.parseFontSize()
	return valid
}

// DefaultKeyDown provides the default key down handling for the font size field.
func (p *FontPanel) DefaultKeyDown(keyCode KeyCode, mod Modifiers, repeat bool) bool {
	if mod.OSMenuCmdModifierDown() {
		return false
	}
	if keyCode == KeyReturn || keyCode == KeyNumPadEnter {
		if v, valid := p.parseFontSize(); valid {
			p.adjustFontSize(v)
		}
		return true
	}
	return p.fontSizeField.DefaultKeyDown(keyCode, mod, repeat)
}

// DefaultFocusLost provides the default focus lost handling for the font size field.
func (p *FontPanel) DefaultFocusLost() {
	if v, valid := p.parseFontSize(); valid {
		p.adjustFontSize(v)
	}
	p.fontSizeField.DefaultFocusLost()
}

func (p *FontPanel) adjustFontSize(value float32) {
	if value != p.fontDescriptor.Size {
		p.fontDescriptor.Size = value
		p.fontModified()
	}
}

func (p *FontPanel) fontModified() {
	if p.FontModifiedCallback != nil {
		toolbox.Call(func() {
			p.FontModifiedCallback(p.fontDescriptor)
		})
	}
}

func (p *FontPanel) parseFontSize() (value float32, valid bool) {
	v, err := strconv.ParseFloat(p.normalizeFontSize(), 32)
	value = float32(v)
	return value, err == nil && value >= p.MinFontSize && value <= p.MaxFontSize
}

func (p *FontPanel) normalizeFontSize() string {
	str := strings.TrimSpace(p.fontSizeField.Text())
	if str == "" {
		str = formatFloat32(p.DefaultFontSize)
	}
	return str
}

func (p *FontPanel) adjustForCurrentFontFamily() {
	family := MatchFontFamily(p.fontDescriptor.Family)
	if family == nil {
		return
	}
	count := family.Count()
	if count == 0 {
		return
	}
	fds := make([]FontFaceDescriptor, 0, count)
	possibleWeights := make(map[weight.Enum]bool)
	possibleSlants := make(map[slant.Enum]bool)
	possibleSpacings := make(map[spacing.Enum]bool)
	for i := 0; i < count; i++ {
		face := family.Face(i)
		w, sp, sl := face.Style()
		possibleWeights[w] = true
		possibleSlants[sl] = true
		possibleSpacings[sp] = true
		fds = append(fds, FontFaceDescriptor{
			Family:  p.fontDescriptor.Family,
			Weight:  w,
			Spacing: sp,
			Slant:   sl,
		})
	}
	bestIndex := 0
	bestScore := -1
	for i, fd := range fds {
		score := 0
		if p.fontDescriptor.Weight == fd.Weight {
			score += 100
		}
		if p.fontDescriptor.Slant == fd.Slant {
			score += 10
		}
		if p.fontDescriptor.Spacing == fd.Spacing {
			score++
		}
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	p.fontDescriptor.FontFaceDescriptor = fds[bestIndex]
	adjustPopupForFont(p.fontWeightPopup, fds[bestIndex].Weight, weight.All, possibleWeights)
	adjustPopupForFont(p.fontSlantPopup, fds[bestIndex].Slant, slant.All, possibleSlants)
	adjustPopupForFont(p.fontSpacingPopup, fds[bestIndex].Spacing, spacing.All, possibleSpacings)
}

func adjustPopupForFont[T comparable](popup *PopupMenu[T], value T, values []T, enablement map[T]bool) {
	for i, one := range values {
		popup.SetItemEnabledAt(i, enablement[one])
	}
	savedCallback := popup.SelectionChangedCallback
	popup.SelectionChangedCallback = nil
	popup.Select(value)
	popup.SelectionChangedCallback = savedCallback
}

func formatFloat32(v float32) string {
	return strconv.FormatFloat(float64(v), 'f', -1, 32)
}
