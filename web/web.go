package web

import "embed"

// Assets şablonları ve statik dosyaları Go ikili dosyasına gömmek için kullanılır.
//
//go:embed templates/* static/*
var Assets embed.FS
