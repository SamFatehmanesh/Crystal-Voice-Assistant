package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	dsp "github.com/asticode/go-astideepspeech"
	"github.com/gen2brain/malgo"
	"io"
	"log"
	"regexp"
	"strings"
	"time"
)

func Handle(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func toint16(data io.Reader) []int16 {
	var x []int16
	var y int16
	for {
		err := binary.Read(data,binary.LittleEndian,&y)
		if errors.Is(err, io.EOF){
			return x
		}
		if y != 0 {
			x = append(x,y)
		}
	}
}


func main() {
	//makes new model object from pretrained deep speech model
	model, err := dsp.New("./deepspeech-0.9.0-models.pbmm")
	Handle(err)
	//gets context for malgo
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		fmt.Printf("LOG <%v>\n", message)
	})
	Handle(err)
	defer func() {
		_ = ctx.Uninit()
		ctx.Free()
	}()

	//configuration for malgo audio
	sp := model.SampleRate()
	fmt.Println(sp)
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = 1
	deviceConfig.SampleRate = uint32(sp)
	deviceConfig.Alsa.NoMMap = 1

	//buffer for the incoming audio data
	var pCapturedSamples []byte
	//a counter for frames of audio
	var capturedSampleCount uint32

	//new audio stream
	stream, err := model.NewStream()
	Handle(err)
	//counter for audio clearing
	i := 0
	//how many lisentime seconds for each check for activation
	lisentime := uint32(1)
	//audio size of bytes
	sizeInBytes := uint32(malgo.SampleSizeInBytes(deviceConfig.Capture.Format))
	//Hold while the program is activated
	hold := false
	//the data stream of words generated by deepspeech stt
	var parsed []string
	//the data stream in the form of a string
	var current string
	//A regex to check for the "computer" activation word
	regex, err := regexp.Compile("compu")
	Handle(err)
	//The main audio loop used to collect
	onRecvFrames := func(pSample2, pSample []byte, framecount uint32) {
		sampleCount := framecount * deviceConfig.Capture.Channels * sizeInBytes
		pCapturedSamples = append(pCapturedSamples, pSample...)
		newCapturedSampleCount := capturedSampleCount + sampleCount
		capturedSampleCount = newCapturedSampleCount
		streamthing := bytes.NewReader(pSample)
		stream.FeedAudioContent(toint16(streamthing))
		if capturedSampleCount > deviceConfig.SampleRate*lisentime {
			current, err = stream.IntermediateDecode()
			Handle(err)
			fmt.Println(current)
			parsed = strings.Split(current," ")
			//fmt.Println(parsed)
			if len(parsed) > 0 {
				if regex.MatchString(parsed[len(parsed)-1]) && !hold {
					oldsize := len(parsed)
					hold = true
					fmt.Println("interpreting...")
					go func(oldsize int,currenttext *[]string, hold *bool) {
						time.Sleep(time.Millisecond*500)
						prevlength := len(*currenttext)
						command := *currenttext
						for {
							if "clear" == command[len(command)-1]{
								*hold = false
								return
							}
							time.Sleep(time.Millisecond*1500)
							command = *currenttext
							if len(command) == prevlength {
								break
							}
							prevlength = len(command)
						}
						command = command[oldsize:]
						//fmt.Println("running " + strings.Join(command," "))
						Interpret(currenttext,oldsize)
						*hold = false
					}(oldsize, &parsed,&hold)
				}
			}
			capturedSampleCount = 0
			i++
		}
		if i == 240 && !hold{
			i = 0
			stream.Discard()
			stream, err = model.NewStream()
			Handle(err)
		}

	}







	fmt.Println("Recording...")
	captureCallbacks := malgo.DeviceCallbacks{
		Data: onRecvFrames,
	}
	device, err := malgo.InitDevice(ctx.Context, deviceConfig, captureCallbacks)
	err = device.Start()
	Handle(err)

	fmt.Scanln()

	device.Uninit()

	Handle(err)
}