// Copyright 2024 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package vecstore

import (
	"slices"

	"github.com/cockroachdb/cockroach/pkg/sql/vecindex/quantize"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/cockroach/pkg/util/vector"
)

// EncodePartitionMetadata encodes the metadata for a partition.
func EncodePartitionMetadata(level Level, centroid vector.T) ([]byte, error) {
	// The encoding consists of 8 bytes for the level, and a 4-byte length,
	// followed by 4 bytes for each dimension in the vector.
	encMetadataSize := 8 + (4 * (len(centroid) + 1))
	buf := make([]byte, 0, encMetadataSize)
	buf = encoding.EncodeUint32Ascending(buf, uint32(level))
	return vector.Encode(buf, centroid)
}

// EncodeRaBitQVector encodes a RaBitQ vector into the given byte slice.
func EncodeRaBitQVector(
	appendTo []byte, codeCount uint32, centroidDistance, dotProduct float32, code quantize.RaBitQCode,
) []byte {
	appendTo = encoding.EncodeUint32Ascending(appendTo, codeCount)
	appendTo = encoding.EncodeUntaggedFloat32Value(appendTo, centroidDistance)
	appendTo = encoding.EncodeUntaggedFloat32Value(appendTo, dotProduct)
	for _, c := range code {
		appendTo = encoding.EncodeUint64Ascending(appendTo, c)
	}
	return appendTo
}

// EncodeUnquantizedVector encodes a full vector into the given byte slice.
func EncodeUnquantizedVector(
	appendTo []byte, centroidDistance float32, v vector.T,
) ([]byte, error) {
	appendTo = encoding.EncodeUntaggedFloat32Value(appendTo, centroidDistance)
	return vector.Encode(appendTo, v)
}

// EncodePartitionKey encodes a partition key into the given byte slice.
func EncodePartitionKey(appendTo []byte, key PartitionKey) []byte {
	return encoding.EncodeUvarintAscending(appendTo, uint64(key))
}

// EncodeChildKey encodes a child key into the given byte slice. The "appendTo"
// slice is expected to be the prefix shared between all KV entries for a
// partition.
func EncodeChildKey(appendTo []byte, key ChildKey) []byte {
	if key.PrimaryKey != nil {
		// The primary key is already in encoded form.
		return append(appendTo, key.PrimaryKey...)
	}
	return EncodePartitionKey(appendTo, key.PartitionKey)
}

// DecodePartitionMetadata decodes the metadata for a partition.
func DecodePartitionMetadata(encMetadata []byte) (level Level, centroid vector.T, err error) {
	encMetadata, decodedLevel, err := encoding.DecodeUint32Ascending(encMetadata)
	if err != nil {
		return 0, nil, err
	}
	centroid, err = vector.Decode(encMetadata)
	if err != nil {
		return 0, nil, err
	}
	return Level(decodedLevel), centroid, nil
}

// DecodeRaBitQVectorToSet decodes a RaBitQ vector entry into the given
// RaBitQuantizedVectorSet. The vector set must have been initialized with the
// correct number of dimensions.
func DecodeRaBitQVectorToSet(encVector []byte, vectorSet *quantize.RaBitQuantizedVectorSet) error {
	encVector, codeCount, err := encoding.DecodeUint32Ascending(encVector)
	if err != nil {
		return err
	}
	encVector, centroidDistance, err := encoding.DecodeUntaggedFloat32Value(encVector)
	if err != nil {
		return err
	}
	encVector, dotProduct, err := encoding.DecodeUntaggedFloat32Value(encVector)
	if err != nil {
		return err
	}
	vectorSet.CodeCounts = append(vectorSet.CodeCounts, codeCount)
	vectorSet.CentroidDistances = append(vectorSet.CentroidDistances, centroidDistance)
	vectorSet.DotProducts = append(vectorSet.DotProducts, dotProduct)
	vectorSet.Codes.Data = slices.Grow(vectorSet.Codes.Data, vectorSet.Codes.Width)
	for i := 0; i < vectorSet.Codes.Width; i++ {
		var codeWord uint64
		encVector, codeWord, err = encoding.DecodeUint64Ascending(encVector)
		if err != nil {
			return err
		}
		vectorSet.Codes.Data = append(vectorSet.Codes.Data, codeWord)
	}
	vectorSet.Codes.Count++
	return nil
}

// DecodeUnquantizedVectorToSet decodes a full vector entry into the given
// UnQuantizedVectorSet. The vector set must have been initialized with the
// correct number of dimensions.
func DecodeUnquantizedVectorToSet(
	encVector []byte, vectorSet *quantize.UnQuantizedVectorSet,
) error {
	encVector, centroidDistance, err := encoding.DecodeUntaggedFloat32Value(encVector)
	if err != nil {
		return err
	}
	v, err := vector.Decode(encVector)
	if err != nil {
		return err
	}
	vectorSet.CentroidDistances = append(vectorSet.CentroidDistances, centroidDistance)
	vectorSet.Vectors.Add(v)
	return nil
}

// DecodeChildKey decodes a child key from the given byte slice.
// NOTE: the returned ChildKey may reference the input slice.
func DecodeChildKey(encChildKey []byte, level Level) (ChildKey, error) {
	if level == LeafLevel {
		// Leaf vectors point to the primary index. The primary key is already in
		// encoded form, so just use it as-is.
		return ChildKey{PrimaryKey: encChildKey}, nil
	} else {
		// Non-leaf vectors point to the partition key.
		_, childPartitionKey, err := encoding.DecodeUvarintAscending(encChildKey)
		if err != nil {
			return ChildKey{}, err
		}
		return ChildKey{PartitionKey: PartitionKey(childPartitionKey)}, nil
	}
}
