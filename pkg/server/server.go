// SPDX-License-Identifier: MIT
// Copyright (c) 2019 Hadrien Chauvin

package server

import (
	"encoding/json"
	"fmt"
	"github.com/hchauvin/name_manager/pkg/name_manager"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
)

func Serve(listener net.Listener, nm name_manager.NameManager) error {
	router := httprouter.New()
	router.GET(
		"/family/:family/$acquire",
		func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			family := p.ByName("family")
			name, err := nm.Acquire(family)
			if err != nil {
				log.WithField("family", family).WithError(err).Error("could not acquire")
				w.WriteHeader(500)
			} else {
				log.WithFields(log.Fields{
					"family": family,
					"name":   name,
				}).Info("name acquired")
				w.WriteHeader(200)
				w.Write([]byte(name))
			}
		})
	router.GET(
		"/family/:family/name/:name/$keep_alive",
		func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			family := p.ByName("family")
			name := p.ByName("name")
			err := nm.KeepAlive(family, name)
			if err != nil {
				log.WithFields(log.Fields{
					"family": family,
					"name":   name,
				}).WithError(err).Error("keep alive errored")
				w.WriteHeader(500)
			} else {
				log.WithFields(log.Fields{
					"family": family,
					"name":   name,
				}).Debug("keep alive")
				w.WriteHeader(200)
			}
		})
	router.GET(
		"/family/:family/name/:name/$release",
		func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			family := p.ByName("family")
			name := p.ByName("name")
			err := nm.Release(family, name)
			if err != nil {
				log.WithFields(log.Fields{
					"family": family,
					"name":   name,
				}).WithError(err).Error("could not release")
				w.WriteHeader(500)
			} else {
				log.WithFields(log.Fields{
					"family": family,
					"name":   name,
				}).Info("name released")
				w.WriteHeader(200)
			}
		})
	router.GET(
		"/family/:family/name/:name/$try_acquire",
		func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			family := p.ByName("family")
			name := p.ByName("name")
			err := nm.TryAcquire(family, name)
			if err == name_manager.ErrNotExist {
				log.WithFields(log.Fields{
					"family":   family,
					"name":     name,
					"response": "ERR_NOT_EXIST",
				}).Info("try acquire")
				w.WriteHeader(200)
				w.Write([]byte("ERR_NOT_EXIST"))
			} else if err == name_manager.ErrInUse {
				log.WithFields(log.Fields{
					"family":   family,
					"name":     name,
					"response": "ERR_IN_USE",
				}).Info("try acquire")
				w.WriteHeader(200)
				w.Write([]byte("ERR_IN_USE"))
			} else if err != nil {
				log.WithFields(log.Fields{
					"family": family,
					"name":   name,
				}).WithError(err).Error("could not try-acquire")
				w.WriteHeader(500)
			} else {
				log.WithFields(log.Fields{
					"family":   family,
					"name":     name,
					"response": "OK",
				}).Info("try acquire")
				w.WriteHeader(200)
				w.Write([]byte("OK"))
			}
		})
	router.GET(
		"/",
		func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			names, err := nm.List()
			if err != nil {
				log.WithError(err).Error("list errored")
				w.WriteHeader(500)
			} else {
				log.Debug("list")
				w.WriteHeader(200)
				b, err := json.MarshalIndent(names, "", "  ")
				if err != nil {
					panic(fmt.Sprintf("%v", err))
				}
				w.Write(b)
			}
		})
	router.GET(
		"/$reset",
		func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			err := nm.Reset()
			if err != nil {
				log.WithError(err).Error("reset errored")
				w.WriteHeader(500)
			} else {
				log.WithError(err).Error("reset")
				w.WriteHeader(200)
			}
		})
	router.GET(
		"/health",
		func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			w.WriteHeader(200)
			w.Write([]byte("OK"))
		})

	return http.Serve(listener, router)
}
