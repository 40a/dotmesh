import models from 'template-ui/lib/plugins/form/models'
import fields from 'template-ui/lib/plugins/form/Components'
import validators from 'template-ui/lib/plugins/form/validators'

const authLogin = {
  username: models.string({
    component: fields.input,
    validate: [validators.required]
  }),
  password: models.string({
    type: 'password',
    component: fields.input,
    validate: validators.required
  })
}

const authRegister = {
  email: models.string({
    component: fields.input,
    validate: [validators.required,validators.email]
  }),
  username: models.string({
    component: fields.input,
    validate: [validators.required]
  }),
  password: models.string({
    type: 'password',
    component: fields.input,
    validate: validators.required
  })
}

const formFields = {
  authLogin,
  authRegister
}

const forms = Object.keys(formFields).reduce((all, field) => {
  all[field] = {
    name: field,
    fields: formFields[field]
  }
  return all
}, {})

export default forms