package registry

import (
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// CloneTypes returns a clone of the given protoregistry.Types registry.
// It may panic if any of the messages, enums or extensions in the given
// registry cannot be added to the clone, however that should never happen.
func CloneTypes(in *protoregistry.Types) *protoregistry.Types {
	out := &protoregistry.Types{}

	in.RangeMessages(func(mt protoreflect.MessageType) bool {
		if err := out.RegisterMessage(mt); err != nil {
			panic(err)
		}
		return true
	})

	in.RangeEnums(func(et protoreflect.EnumType) bool {
		if err := out.RegisterEnum(et); err != nil {
			panic(err)
		}
		return true
	})

	in.RangeExtensions(func(et protoreflect.ExtensionType) bool {
		if err := out.RegisterExtension(et); err != nil {
			panic(err)
		}
		return true
	})

	return out
}

// AddDynamicTypes adds dynamicpb types to the given Types registry for all
// Messages, Enums and Extensions in the given FileDescriptorSet. If a type
// already exists in the Types registry, the dynamic type will not be added
// to replace it, and instead will be ignored.
func AddDynamicTypes(t *protoregistry.Types, fds *descriptorpb.FileDescriptorSet) error {
	// FileDescriptor and MessageDescriptor implement the typesContainer interface
	type typesContainer interface {
		Messages() protoreflect.MessageDescriptors
		Enums() protoreflect.EnumDescriptors
		Extensions() protoreflect.ExtensionDescriptors
	}

	// addTypes ignores errors from the Types.Register* methods, assuming that
	// a concrete type is already registered. Concrete types take precedence
	// over dynamic types as they are compiled into the binary and are more
	// useful and more easily operated upon.
	var addTypes func(tc typesContainer)
	addTypes = func(tc typesContainer) {
		mds := tc.Messages()
		for i := 0; i < mds.Len(); i++ {
			md := mds.Get(i)
			t.RegisterMessage(dynamicpb.NewMessageType(md)) //nolint:errcheck
			addTypes(md)
		}

		enumds := tc.Enums()
		for i := 0; i < enumds.Len(); i++ {
			t.RegisterEnum(dynamicpb.NewEnumType(enumds.Get(i))) //nolint:errcheck
		}

		extds := tc.Extensions()
		for i := 0; i < extds.Len(); i++ {
			t.RegisterExtension(dynamicpb.NewExtensionType(extds.Get(i))) //nolint:errcheck
		}
	}

	files, err := protodesc.FileOptions{AllowUnresolvable: true}.NewFiles(fds)
	if err != nil {
		return err
	}
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		addTypes(fd)
		return true
	})
	return nil
}
