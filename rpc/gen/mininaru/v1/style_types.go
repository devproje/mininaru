// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package mininaruv1

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
)

type messageState interface {
	protoreflect.Message

	StoreMessageInfo(*protoimpl.MessageInfo)
	LoadMessageInfo() *protoimpl.MessageInfo
}
