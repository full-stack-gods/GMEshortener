package web

import (
	"github.com/gofiber/fiber/v2"
	"log"
)

func _errResponse(ctx *fiber.Ctx, status int, err ...interface{}) error {
	// default error message
	message := "unspecified error"
	if len(err) > 0 {
		switch v := err[0].(type) {
		case error:
			message = v.Error()
			break
		case string:
			message = v
			break
		default:
			log.Println("invalid type")
		}
	}
	return ctx.Status(status).JSON(&Successable{
		Success: false,
		Message: message,
	})
}

func UserErrorResponse(ctx *fiber.Ctx, err ...interface{}) error {
	return _errResponse(ctx, 400, err...)
}

func ServerErrorResponse(ctx *fiber.Ctx, err ...interface{}) error {
	return _errResponse(ctx, 500, err...)
}

func SuccessResponse(ctx *fiber.Ctx) error {
	return ctx.Status(200).JSON(&Successable{
		Success: true,
		Message: "success",
	})
}

func SuccessDataResponse(ctx *fiber.Ctx, data interface{}) error {
	return ctx.Status(200).JSON(&Successable{
		Success: true,
		Message: "success",
		Data:    data,
	})
}

func SuccessMessageResponse(ctx *fiber.Ctx, message string) error {
	return ctx.Status(200).JSON(&Successable{
		Success: true,
		Message: message,
	})
}

func SuccessMessageDataResponse(ctx *fiber.Ctx, message string, data interface{}) error {
	return ctx.Status(200).JSON(&Successable{
		Success: true,
		Message: message,
		Data:    data,
	})
}
