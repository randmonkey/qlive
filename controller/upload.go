// Copyright 2020 Qiniu Cloud (qiniu.com)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"github.com/qiniu/api.v7/v7/auth"
	"github.com/qiniu/api.v7/v7/storage"
	"github.com/qiniu/x/xlog"
	"github.com/qrtc/qlive/config"
)

// QiniuUploadController 七牛对象存储提供的上传服务。
type QiniuUploadController struct {
	xl *xlog.Logger
	// 文件所在的bucket。
	Bucket      string
	Credentials auth.Credentials
}

// TODO: 上传文件信息存储到数据库，可查看用户上传token记录或上传文件记录？

// NewQiniuUploadController 创建七牛对象存储上传服务。
func NewQiniuUploadController(conf *config.QiniuStorageConfig, xl *xlog.Logger) (*QiniuUploadController, error) {
	if xl == nil {
		xl = xlog.New("qiniu-upload-service")
	}
	return &QiniuUploadController{
		xl:     xl,
		Bucket: conf.Bucket,
		Credentials: auth.Credentials{
			AccessKey: conf.KeyPair.AccessKey,
			SecretKey: []byte(conf.KeyPair.SecretKey),
		},
	}, nil
}

// GetUploadToken 获取上传文件到七牛对象存储的token。
func (c *QiniuUploadController) GetUploadToken(xl *xlog.Logger, userID string, filename string, expireSeconds int) (string, error) {
	if xl == nil {
		xl = c.xl
	}
	putPolicy := &storage.PutPolicy{
		Scope:      c.Bucket + ":" + filename,
		Expires:    uint64(expireSeconds),
		InsertOnly: 1,
		EndUser:    userID,
	}
	return putPolicy.UploadToken(&c.Credentials), nil
}
