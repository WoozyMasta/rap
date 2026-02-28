# rap

Go library for decoding and encoding  
Real Virtuality / DayZ RAP binary config format (`.bin`, binary `.rvmat`).

* RAP decode to `rvcfg` AST.
* RAP encode from `rvcfg` AST.
* Scalar subtype handling (string / float / long) with float normalization.
* Class-body offsets and nested array support.

## Install

```bash
go get github.com/woozymasta/rap
```

## Relationship with `rvcfg`

`rap` uses `github.com/woozymasta/rvcfg` as text frontend:

* preprocess + parse source text into AST
* encode AST to RAP binary
* decode RAP binary back to AST/text

## Usage

Parse source and encode RAP:

```go
parsed, err := rap.ParseSourceFileWithDefaults("config.cpp")
if err != nil {
  // handle
}

// or pass explicit options:
parsed, err := rap.ParseSourceFile("config.cpp", rap.SourceParseOptions{
  Preprocess: rvcfg.PreprocessOptions{
    IncludeDirs: []string{"./include"},
  },
  Parse: rvcfg.ParseOptions{
    CaptureScalarRaw: true,
  },
})
if err != nil {
  // handle
}

bin, err := rap.EncodeAST(parsed.Processed.Parse.File, rap.EncodeOptions{})
```

Encode RAP directly from in-memory source (`[]byte`):

```go
bin, err := rap.EncodeBytesWithDefaults(
  "config.cpp",
  []byte(`class CfgPatches { class TestMod { units[] = {}; }; };`),
)
if err != nil {
  // handle
}
```

Decode RAP to AST:

```go
file, err := rap.DecodeToAST(data, rap.DecodeOptions{})
if err != nil {
  // handle
}

_ = file.Statements
```

Decode RAP to text:

```go
text, err := rap.DecodeToText(data, rap.DecodeOptions{}, rap.RenderOptions{
  Format: rvcfg.FormatOptions{
    MaxLineWidth: 120,
  },
})
```

Decode RAP from file path:

```go
file, err := rap.DecodeFile("config.bin", rap.DecodeOptions{})
text, err := rap.DecodeFileToText("config.bin", rap.DecodeOptions{}, rap.RenderOptions{})
```

## Decode options

```go
rap.DecodeOptions{
  DisableFloatNormalization: false, // default: shortest stable float32 text
}
```

## Format coverage

Implemented RAP entry types:

* `0` class with body offset
* `1` scalar assignment with float value
* `2` array assignment with int32 value
* `3` extern class
* `4` delete
* `5` array append (`+=`)
* `6` scalar assignment with int64 value

## References

* <https://community.bistudio.com/wiki/raP_File_Format_-_Elite>
* <https://community.bistudio.com/wiki/Config.cpp/bin_File_Format>
* <https://community.bistudio.com/wiki/TokenNameValueTypes>
