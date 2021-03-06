// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2019 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package messages

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	"maunium.net/go/gomuks/matrix/event"
	"maunium.net/go/mauview"
	"maunium.net/go/tcell"

	"maunium.net/go/gomuks/config"
	"maunium.net/go/gomuks/debug"
	"maunium.net/go/gomuks/interface"
	"maunium.net/go/gomuks/lib/ansimage"
	"maunium.net/go/gomuks/ui/messages/tstring"
)

type ImageMessage struct {
	Body       string
	Homeserver string
	FileID     string
	data       []byte
	buffer     []tstring.TString

	matrix ifc.MatrixContainer
}

// NewImageMessage creates a new ImageMessage object with the provided values and the default state.
func NewImageMessage(matrix ifc.MatrixContainer, evt *event.Event, displayname string, body, homeserver, fileID string, data []byte) *UIMessage {
	return newUIMessage(evt, displayname, &ImageMessage{
		Body: body,
		Homeserver: homeserver,
		FileID: fileID,
		data: data,
		matrix: matrix,
	})
}

func (msg *ImageMessage) Clone() MessageRenderer {
	data := make([]byte, len(msg.data))
	copy(data, msg.data)
	return &ImageMessage{
		Body:        msg.Body,
		Homeserver:  msg.Homeserver,
		FileID:      msg.FileID,
		data:        data,
		matrix:      msg.matrix,
	}
}

func (msg *ImageMessage) RegisterMatrix(matrix ifc.MatrixContainer) {
	msg.matrix = matrix

	if len(msg.data) == 0 {
		go msg.updateData()
	}
}

func (msg *ImageMessage) NotificationContent() string {
	return "Sent an image"
}

func (msg *ImageMessage) PlainText() string {
	return fmt.Sprintf("%s: %s", msg.Body, msg.matrix.GetDownloadURL(msg.Homeserver, msg.FileID))
}

func (msg *ImageMessage) String() string {
	return fmt.Sprintf(`&messages.ImageMessage{Body="%s", Homeserver="%s", FileID="%s"}`, msg.Body, msg.Homeserver, msg.FileID)
}

func (msg *ImageMessage) updateData() {
	defer debug.Recover()
	debug.Print("Loading image:", msg.Homeserver, msg.FileID)
	data, _, _, err := msg.matrix.Download(fmt.Sprintf("mxc://%s/%s", msg.Homeserver, msg.FileID))
	if err != nil {
		debug.Printf("Failed to download image %s/%s: %v", msg.Homeserver, msg.FileID, err)
		return
	}
	debug.Print("Image", msg.Homeserver, msg.FileID, "loaded.")
	msg.data = data
}

func (msg *ImageMessage) Path() string {
	return msg.matrix.GetCachePath(msg.Homeserver, msg.FileID)
}

// CalculateBuffer generates the internal buffer for this message that consists
// of the text of this message split into lines at most as wide as the width
// parameter. If the message width is larger than the width of the buffer
// the message is scaled to one third the buffer width.
func (msg *ImageMessage) CalculateBuffer(prefs config.UserPreferences, width int, uiMsg *UIMessage) {
	if width < 2 {
		return
	}

	if prefs.BareMessageView || prefs.DisableImages {
		msg.buffer = calculateBufferWithText(prefs, tstring.NewTString(msg.PlainText()), width, uiMsg)
		return
	}

	img, _, err := image.DecodeConfig(bytes.NewReader(msg.data))
	if err != nil {
		debug.Print("Image could not be decoded:", err)
	}
	imgWidth := img.Width
	if img.Width > width {
		imgWidth = width / 3
	}

	ansImage, err := ansimage.NewScaledFromReader(bytes.NewReader(msg.data), 0, imgWidth, color.Black)
	if err != nil {
		msg.buffer = []tstring.TString{tstring.NewColorTString("Failed to display image", tcell.ColorRed)}
		debug.Print("Failed to display image:", err)
		return
	}

	msg.buffer = ansImage.Render()
}

func (msg *ImageMessage) Height() int {
	return len(msg.buffer)
}

func (msg *ImageMessage) Draw(screen mauview.Screen) {
	for y, line := range msg.buffer {
		line.Draw(screen, 0, y)
	}
}
